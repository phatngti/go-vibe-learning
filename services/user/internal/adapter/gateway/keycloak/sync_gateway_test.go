package keycloak

import (
	"context"
	"testing"

	"github.com/Nerzal/gocloak/v13"
)

type syncKeycloakClientStub struct {
	users  []*gocloak.User
	params gocloak.GetUsersParams
}

func (*syncKeycloakClientStub) RegisterUser(context.Context, gocloak.User) (string, error) {
	return "", nil
}
func (*syncKeycloakClientStub) LoginUser(context.Context, string, string) (*gocloak.JWT, error) {
	return nil, nil
}
func (*syncKeycloakClientStub) RefreshToken(context.Context, string) (*gocloak.JWT, error) {
	return nil, nil
}
func (s *syncKeycloakClientStub) GetUsers(_ context.Context, params gocloak.GetUsersParams) ([]*gocloak.User, error) {
	s.params = params
	return s.users, nil
}
func (*syncKeycloakClientStub) GetUserByID(context.Context, string) (*gocloak.User, error) {
	return nil, nil
}

func TestFetchUsersPageUsesBoundedKeycloakPagination(t *testing.T) {
	id1, username1 := "provider-11", "user-11"
	id2, username2 := "provider-12", "user-12"
	client := &syncKeycloakClientStub{users: []*gocloak.User{
		{ID: &id1, Username: &username1}, {ID: &id2, Username: &username2},
	}}
	gateway := NewSyncGateway(client)
	page, err := gateway.FetchUsersPage(context.Background(), 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if client.params.First == nil || *client.params.First != 10 || client.params.Max == nil || *client.params.Max != 2 {
		t.Fatalf("Keycloak pagination params = %+v", client.params)
	}
	if len(page.Users) != 2 || page.NextCursor != 12 || page.Done {
		t.Fatalf("page = %+v", page)
	}
}

func TestFetchUsersPageMarksShortPageComplete(t *testing.T) {
	id, username := "provider-20", "user-20"
	client := &syncKeycloakClientStub{users: []*gocloak.User{{ID: &id, Username: &username}}}
	gateway := NewSyncGateway(client)
	page, err := gateway.FetchUsersPage(context.Background(), 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Done || page.NextCursor != 21 {
		t.Fatalf("page = %+v", page)
	}
}
