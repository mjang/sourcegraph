package scim

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/elimity-com/scim"
	"github.com/scim2/filter-parser/v2"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestUserResourceHandler_Create(t *testing.T) {
	db := getMockDB()
	userResourceHandler := NewUserResourceHandler(context.Background(), &observation.TestContext, db)
	user, err := userResourceHandler.Create(&http.Request{}, scim.ResourceAttributes{
		"userName": "user1",
		"name": map[string]interface{}{
			"givenName":  "First",
			"middleName": "Middle",
			"familyName": "Last",
		},
		"emails": []interface{}{
			map[string]interface{}{
				"value":   "a@b.c",
				"primary": true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert that ID is correct
	assert.Equal(t, "5", user.ID)
}

func TestUserResourceHandler_Get(t *testing.T) {
	db := getMockDB()
	userResourceHandler := NewUserResourceHandler(context.Background(), &observation.TestContext, db)
	user1, err := userResourceHandler.Get(&http.Request{}, "1")
	if err != nil {
		t.Fatal(err)
	}
	user2, err := userResourceHandler.Get(&http.Request{}, "2")
	if err != nil {
		t.Fatal(err)
	}

	// Assert that IDs are correct
	if user1.ID != "1" {
		t.Errorf("expected ID = 1, got %s", user1.ID)
	}
	if user2.ID != "2" {
		t.Errorf("expected ID = 2, got %s", user2.ID)
	}
	if user1.ExternalID.Value() != "external1" {
		t.Errorf("expected ExternalID = 'external1', got %s", user1.ExternalID.Value())
	}
	if user2.ExternalID.Value() != "" {
		t.Errorf("expected no ExternalID, got %s", user1.ExternalID.Value())
	}

	// Assert that usernames are correct
	if user1.Attributes["userName"] != "user1" {
		t.Errorf("expected username = 'user1', got %s", user1.Attributes["UserName"])
	}
	if user2.Attributes["userName"] != "user2" {
		t.Errorf("expected username = 'user2', got %s", user2.Attributes["UserName"])
	}

	// Assert that names are correct
	if user1.Attributes["displayName"] != "First Last" {
		t.Errorf("expected First Last, got %s", user1.Attributes["displayName"])
	}
	if user2.Attributes["displayName"] != "First Middle Last" {
		t.Errorf("expected First Middle Last, got %s", user2.Attributes["displayName"])
	}
	if user1.Attributes["name"].(map[string]interface{})["givenName"] != "First" {
		t.Errorf("expected First, got %s", user1.Attributes["name"].(map[string]interface{})["givenName"])
	}
	if user1.Attributes["name"].(map[string]interface{})["middleName"] != "" {
		t.Errorf("expected empty string, got %s", user1.Attributes["name"].(map[string]interface{})["middleName"])
	}
	if user1.Attributes["name"].(map[string]interface{})["familyName"] != "Last" {
		t.Errorf("expected Last, got %s", user1.Attributes["name"].(map[string]interface{})["familyName"])
	}
	if user2.Attributes["name"].(map[string]interface{})["givenName"] != "First" {
		t.Errorf("expected First, got %s", user2.Attributes["name"].(map[string]interface{})["givenName"])
	}
	if user2.Attributes["name"].(map[string]interface{})["middleName"] != "Middle" {
		t.Errorf("expected Middle, got %s", user2.Attributes["name"].(map[string]interface{})["middleName"])
	}
	if user2.Attributes["name"].(map[string]interface{})["familyName"] != "Last" {
		t.Errorf("expected Last, got %s", user2.Attributes["name"].(map[string]interface{})["familyName"])
	}

	// Assert that emails are correct
	if user1.Attributes["emails"].([]interface{})[0].(map[string]interface{})["value"] != "a@example.com" {
		t.Errorf("expected empty email, got %s", user1.Attributes["emails"].([]interface{})[0].(map[string]interface{})["value"])
	}
}

func TestUserResourceHandler_GetAll(t *testing.T) {
	db := getMockDB()

	cases := []struct {
		name             string
		count            int
		startIndex       int
		filter           string
		wantTotalResults int
		wantResults      int
		wantFirstID      int
	}{
		{name: "no filter, count=0", count: 0, startIndex: 1, filter: "", wantTotalResults: 4, wantResults: 0, wantFirstID: 0},
		{name: "no filter, count=2", count: 2, startIndex: 1, filter: "", wantTotalResults: 4, wantResults: 2, wantFirstID: 1},
		{name: "no filter, offset=3", count: 999, startIndex: 4, filter: "", wantTotalResults: 4, wantResults: 1, wantFirstID: 4},
		{name: "no filter, count=2, offset=1", count: 2, startIndex: 2, filter: "", wantTotalResults: 4, wantResults: 2, wantFirstID: 2},
		{name: "no filter, count=999", count: 999, startIndex: 1, filter: "", wantTotalResults: 4, wantResults: 4, wantFirstID: 1},
		{name: "filter, count=0", count: 0, startIndex: 1, filter: "userName eq \"user3\"", wantTotalResults: 1, wantResults: 0, wantFirstID: 0},
		{name: "filter: userName", count: 999, startIndex: 1, filter: "userName eq \"user3\"", wantTotalResults: 1, wantResults: 1, wantFirstID: 3},
		{name: "filter: OR", count: 999, startIndex: 1, filter: "(userName eq \"user3\") OR (displayName eq \"First Middle Last\")", wantTotalResults: 2, wantResults: 2, wantFirstID: 2},
		{name: "filter: AND", count: 999, startIndex: 1, filter: "(userName eq \"user3\") AND (displayName eq \"First Last\")", wantTotalResults: 1, wantResults: 1, wantFirstID: 3},
	}

	userResourceHandler := NewUserResourceHandler(context.Background(), &observation.TestContext, db)
	for _, c := range cases {
		t.Run("TestUserResourceHandler_GetAll "+c.name, func(t *testing.T) {
			var params scim.ListRequestParams
			if c.filter != "" {
				filterExpr, err := filter.ParseFilter([]byte(c.filter))
				if err != nil {
					t.Fatal(err)
				}
				params = scim.ListRequestParams{Count: c.count, StartIndex: c.startIndex, Filter: filterExpr}
			} else {
				params = scim.ListRequestParams{Count: c.count, StartIndex: c.startIndex, Filter: nil}
			}
			page, err := userResourceHandler.GetAll(&http.Request{}, params)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, c.wantTotalResults, page.TotalResults)
			assert.Equal(t, c.wantResults, len(page.Resources))
			if c.wantResults > 0 {
				assert.Equal(t, strconv.Itoa(c.wantFirstID), page.Resources[0].ID)
			}
		})
	}
}

func getMockDB() *database.MockDB {
	users := []*types.UserForSCIM{
		{User: types.User{ID: 1, Username: "user1", DisplayName: "First Last"}, Emails: []string{"a@example.com"}, SCIMExternalID: "external1"},
		{User: types.User{ID: 2, Username: "user2", DisplayName: "First Middle Last"}, Emails: []string{"b@example.com"}, SCIMExternalID: ""},
		{User: types.User{ID: 3, Username: "user3", DisplayName: "First Last"}},
		{User: types.User{ID: 4, Username: "user4"}},
	}

	userStore := database.NewMockUserStore()
	userStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	userStore.ListForSCIMFunc.SetDefaultHook(func(ctx context.Context, opt *database.UsersListOptions) ([]*types.UserForSCIM, error) {
		// Return the users with the given IDs
		if opt.UserIDs != nil {
			var filteredUsers []*types.UserForSCIM
			for _, id := range opt.UserIDs {
				for _, user := range users {
					if user.ID == id {
						filteredUsers = append(filteredUsers, user)
					}
				}
			}
			return applyLimitOffset(filteredUsers, opt.LimitOffset)
		}

		return applyLimitOffset(users, opt.LimitOffset)
	})
	userStore.CountFunc.SetDefaultReturn(4, nil)
	userStore.CreateFunc.SetDefaultHook(func(ctx context.Context, user database.NewUser) (*types.User, error) {
		return &types.User{ID: 5, Username: user.Username, DisplayName: user.DisplayName}, nil
	})

	// Create DB
	db := database.NewMockDB()
	db.UsersFunc.SetDefaultReturn(userStore)
	return db
}

func applyLimitOffset(users []*types.UserForSCIM, limitOffset *database.LimitOffset) ([]*types.UserForSCIM, error) {
	// Return all users
	if limitOffset == nil {
		return users, nil
	}

	// Return a slice of users based on the limit and offset
	start := limitOffset.Offset
	end := start + limitOffset.Limit
	if end > len(users) {
		end = len(users)
	}
	return users[start:end], nil
}
