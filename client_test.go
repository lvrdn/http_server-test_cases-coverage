package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type Users struct {
	Version string
	List    []User
}

func (u *Users) UnmarshalExtraXML(data []byte) error {

	type realUser struct {
		Id        int    `xml:"id"`
		FirstName string `xml:"first_name"`
		LastName  string `xml:"last_name"`
		Age       int    `xml:"age"`
		About     string `xml:"about"`
		Gender    string `xml:"gender"`
	}

	type realUsers struct {
		Version string     `xml:"version,attr"`
		List    []realUser `xml:"row"`
	}

	realUsersFromFile := &realUsers{}

	if err := xml.Unmarshal(data, realUsersFromFile); err != nil {
		return err
	}

	for _, user := range realUsersFromFile.List {

		newUser := &User{}
		newUser.Name = user.FirstName + " " + user.LastName
		newUser.Id = user.Id
		newUser.Gender = user.Gender
		newUser.Age = user.Age
		newUser.About = user.About

		u.List = append(u.List, *newUser)
	}

	return nil
}

type HandlerStruct struct {
	Users *Users
}

func (h *HandlerStruct) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var accessToken = "123qwerty"

	if accessTokenFromReq := r.Header.Get("AccessToken"); accessToken != accessTokenFromReq {
		w.WriteHeader(401)
		return
	}

	query := r.URL.Query().Get("query")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	order_field := r.URL.Query().Get("order_field")
	order_by, _ := strconv.Atoi(r.URL.Query().Get("order_by"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	switch order_field {
	case "Name", "", "Age", "Id":
	default:
		w.WriteHeader(400)
		Error := map[string]string{"Error": "ErrorBadOrderField"}
		resultError, err := json.Marshal(Error)
		if err != nil {
			fmt.Println(err, "(error with marchal resultError json)")
			return
		}
		w.Write(resultError)
		return
	}

	switch order_by {
	case OrderByAsIs, OrderByAsc, OrderByDesc:
	default:
		w.WriteHeader(400)
		Error := map[string]string{"Error": "order_by must be 1, -1 or 0"}
		resultError, err := json.Marshal(Error)
		if err != nil {
			fmt.Println(err, "(error with marchal resultError json)")
			return
		}
		w.Write(resultError)
		return
	}

	findUsers := []User{}

	for _, user := range h.Users.List {
		if ok := strings.Contains(user.Name, query) || strings.Contains(user.About, query); ok {
			findUsers = append(findUsers, user)
		}
	}

	switch order_field {
	case "Name", "":
		switch order_by {
		case 1:
			sort.Slice(findUsers, func(i, j int) bool {
				return findUsers[i].Name < findUsers[j].Name
			})
		case -1:
			sort.Slice(findUsers, func(i, j int) bool {
				return findUsers[i].Name > findUsers[j].Name
			})
		case 0:
		}

	case "Id":
		switch order_by {
		case 1:
			sort.Slice(findUsers, func(i, j int) bool {
				return findUsers[i].Id < findUsers[j].Id
			})
		case -1:
			sort.Slice(findUsers, func(i, j int) bool {
				return findUsers[i].Id > findUsers[j].Id
			})
		case 0:
		}

	case "Age":
		switch order_by {
		case 1:
			sort.Slice(findUsers, func(i, j int) bool {
				return findUsers[i].Age < findUsers[j].Age
			})
		case -1:
			sort.Slice(findUsers, func(i, j int) bool {
				return findUsers[i].Age > findUsers[j].Age
			})
		case 0:
		}
	}

	if offset >= len(findUsers) {
		w.WriteHeader(400)
		Error := map[string]string{"Error": "offset > number of users found, need to use smaller value"}
		resultError, err := json.Marshal(Error)
		if err != nil {
			fmt.Println(err, "(error with marchal resultError json)")
			return
		}
		w.Write(resultError)
		return
	} else {
		findUsers = findUsers[offset:]
	}

	if limit <= len(findUsers) {
		findUsers = findUsers[:limit]
	}

	resultBytes, err := json.Marshal(findUsers)
	if err != nil {
		fmt.Println(err, "(error with marchal result json)")
		return
	}
	w.Write(resultBytes)
}

func SearchServer() *httptest.Server {

	file, err := os.Open("dataset.xml")

	if err != nil {
		panic(err)
	}
	defer file.Close()

	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	UsersFromXML := &Users{}

	err = UsersFromXML.UnmarshalExtraXML(fileContents)
	if err != nil {
		fmt.Println("Error happened on UnmarshalExtraXML", err)
	}

	usersHandler := &HandlerStruct{Users: UsersFromXML}

	fmt.Println("starting server at: 8080")

	return httptest.NewServer(http.HandlerFunc(usersHandler.ServeHTTP))
}

type TestCase struct {
	TestSearchRequest *SearchRequest
	TestSearchClient  *SearchClient
	isError           bool
	ExpectedError     string
	ExpectedResult    *SearchResponse
}

func SearchServerErrors(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	switch query {
	case "so long time for response":
		time.Sleep(5 * time.Second)
		return
	case "500 error case":
		w.WriteHeader(500)
		return
	case "something for json with error":
		w.WriteHeader(400)
		w.Write([]byte(query))
		return
	case "something for json with result":
		w.Write([]byte(query))
		return
	}

}
func TestServerErrors(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(SearchServerErrors))

	cases := []TestCase{
		{ //1.0 case. Проверка ошибки 500
			TestSearchRequest: &SearchRequest{
				Query: "500 error case",
			},
			TestSearchClient: &SearchClient{
				URL: ts.URL,
			},
			isError:       true,
			ExpectedError: "SearchServer fatal error",
		},
		{ //1.1 case. Проверка ошибки по долгому времени ожидания
			TestSearchRequest: &SearchRequest{
				Query: "so long time for response",
			},
			TestSearchClient: &SearchClient{
				URL: ts.URL,
			},
			isError:       true,
			ExpectedError: "timeout for limit=1&offset=0&order_by=0&order_field=&query=so+long+time+for+response",
		},
		{ //1.2 case. Проверка ошибки по распаковке json с ошибкой
			TestSearchRequest: &SearchRequest{
				Query: "something for json with error",
			},
			TestSearchClient: &SearchClient{
				URL: ts.URL,
			},
			isError:       true,
			ExpectedError: "cant unpack error json: invalid character 's' looking for beginning of value",
		},
		{ //1.3 case. Проверка ошибки по распаковке json с ошибкой
			TestSearchRequest: &SearchRequest{
				Query: "something for json with result",
			},
			TestSearchClient: &SearchClient{
				URL: ts.URL,
			},
			isError:       true,
			ExpectedError: "cant unpack result json: invalid character 's' looking for beginning of value",
		},
	}

	for numCase, oneCase := range cases {
		_, err := oneCase.TestSearchClient.FindUsers(*oneCase.TestSearchRequest)

		if err.Error() != oneCase.ExpectedError && oneCase.isError {
			t.Errorf("test case № [%v] false, expected: %v, got: %v\n", numCase, oneCase.ExpectedError, err)
		}
	}

	ts.Close()

}

func TestFuncSearchClient(t *testing.T) {

	ts := SearchServer()
	cases := []TestCase{
		{ //2.0 case. Проверка вывода функции
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      3,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     4,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:        false,
			ExpectedResult: response_2_0_case,
		},
		{ //2.1 case. Проверка ошибки по limit < 0
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      -5,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     4,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:       true,
			ExpectedError: "limit must be > 0",
		},
		{ //2.2 case. Проверка ошибки по offset < 0
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      5,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     -5,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:       true,
			ExpectedError: "offset must be > 0",
		},
		{ //2.3 case. Проверка ошибки по неправильному токену
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      5,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     5,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "WRONG TOKEN",
				URL:         ts.URL,
			},
			isError:       true,
			ExpectedError: "Bad AccessToken",
		},
		{ //2.4 case. Проверка ошибки по неправильному URL
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      5,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     5,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         "wrongurl",
			},
			isError:       true,
			ExpectedError: "unknown error Get \"wrongurl?limit=6&offset=5&order_by=-1&order_field=Age&query=ill\": unsupported protocol scheme \"\"",
		},
		{ //2.5 case. Проверка ошибки по неправильному OrderField
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      5,
				OrderField: "Ag1111e",
				OrderBy:    -1,
				Offset:     5,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:       true,
			ExpectedError: "OrderFeld Ag1111e invalid",
		},
		{ //2.6 case. Проверка ошибки по неправильному OrderBy
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      5,
				OrderField: "Age",
				OrderBy:    -100,
				Offset:     5,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:       true,
			ExpectedError: "unknown bad request error: order_by must be 1, -1 or 0",
		},
		{ //2.7 case. Проверка ошибки по большому знчению offset, из-за которого выводится 0 пользователей
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      5,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     1000,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:       true,
			ExpectedError: "unknown bad request error: offset > number of users found, need to use smaller value",
		},
		{ //2.8 case. Проверка условия limit > 25
			TestSearchRequest: &SearchRequest{
				Query:      "l",
				Limit:      35,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     5,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:        false,
			ExpectedResult: response_2_8_case,
		},
		{ //2.9 case. Проверка условия limit > len(Users)
			TestSearchRequest: &SearchRequest{
				Query:      "ill",
				Limit:      20,
				OrderField: "Age",
				OrderBy:    -1,
				Offset:     5,
			},
			TestSearchClient: &SearchClient{
				AccessToken: "123qwerty",
				URL:         ts.URL,
			},
			isError:        false,
			ExpectedResult: response_2_9_case,
		},
	}

	for numCase, oneCase := range cases {
		result, err := oneCase.TestSearchClient.FindUsers(*oneCase.TestSearchRequest)

		if err == nil && !oneCase.isError {
			if len(result.Users) == len(oneCase.ExpectedResult.Users) {
				fl := true
				for i, user := range result.Users {
					if user != oneCase.ExpectedResult.Users[i] {
						fl = false
						break
					}
				}
				if !fl {
					t.Errorf("test case № [%v] false, expected and got results mismatch", numCase)
					continue
				}
			} else {
				t.Errorf("test case № [%v] false, expected and got results mismatch", numCase)
				continue
			}
			if result.NextPage != oneCase.ExpectedResult.NextPage {
				t.Errorf("test case № [%v] false, expected and got results mismatch", numCase)
				continue
			}

		} else if err.Error() != oneCase.ExpectedError && oneCase.isError {
			t.Errorf("test case № [%v] false, expected: %v, got: %v\n", numCase, oneCase.ExpectedError, err)
		}
	}
	ts.Close()
}
