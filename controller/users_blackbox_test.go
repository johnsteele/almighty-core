package controller_test

import (
	"fmt"
	"testing"

	"github.com/almighty/almighty-core/account"
	"github.com/almighty/almighty-core/app"
	"github.com/almighty/almighty-core/app/test"
	. "github.com/almighty/almighty-core/controller"
	"github.com/almighty/almighty-core/gormapplication"
	"github.com/almighty/almighty-core/gormsupport"
	"github.com/almighty/almighty-core/gormsupport/cleaner"
	"github.com/almighty/almighty-core/resource"
	testsupport "github.com/almighty/almighty-core/test"
	almtoken "github.com/almighty/almighty-core/token"
	"github.com/goadesign/goa"
	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

func TestUsers(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &TestUsersSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func createSecureController(t *testing.T, identity account.Identity) (*UsersController, *goa.Service) {
	priv, _ := almtoken.ParsePrivateKey([]byte(almtoken.RSAPrivateKey))
	svc := testsupport.ServiceAsUser("Users-Service", almtoken.NewManagerWithPrivateKey(priv), identity)
	app := gormapplication.NewGormDB(DB)
	controller := NewUsersController(svc, app)
	assert.NotNil(t, controller)
	return controller, svc
}

type TestUsersSuite struct {
	gormsupport.DBTestSuite
	db           *gormapplication.GormDB
	svc          *goa.Service
	clean        func()
	controller   *UsersController
	userRepo     account.UserRepository
	identityRepo account.IdentityRepository
}

func (s *TestUsersSuite) SetupSuite() {
	var err error
	s.DB, err = gorm.Open("postgres", wibConfiguration.GetPostgresConfigString())
	s.DB.LogMode(true)
	require.Nil(s.T(), err)
	s.clean = cleaner.DeleteCreatedEntities(s.DB)
	s.svc = goa.New("test")
	app := gormapplication.NewGormDB(s.DB)
	s.controller = NewUsersController(s.svc, app)
	s.userRepo = app.Users()
	s.identityRepo = app.Identities()
}

func (s *TestUsersSuite) TearDownTest() {
	s.clean()
}

func (s *TestUsersSuite) TestUpdateUserOK() {
	// given
	user := s.createRandomUser()
	identity := s.createRandomIdentity(user, account.KeycloakIDP)
	_, result := test.ShowUsersOK(s.T(), nil, nil, s.controller, identity.ID.String())
	assert.Equal(s.T(), identity.ID.String(), *result.Data.ID)
	assert.Equal(s.T(), user.FullName, *result.Data.Attributes.FullName)
	assert.Equal(s.T(), user.ImageURL, *result.Data.Attributes.ImageURL)
	assert.Equal(s.T(), identity.ProviderType, *result.Data.Attributes.ProviderType)
	assert.Equal(s.T(), identity.Username, *result.Data.Attributes.Username)
	// when
	newEmail := "updated-" + uuid.NewV4().String() + "@email.com"
	newFullName := "newFull Name"
	newImageURL := "http://new.image.io/imageurl"
	newBio := "new bio"
	newProfileURL := "http://new.profile.url/url"
	secureController, secureService := createSecureController(s.T(), identity)
	updateUsersPayload := createUpdateUsersPayload(&newEmail, &newFullName, &newBio, &newImageURL, &newProfileURL)
	_, result = test.UpdateUsersOK(s.T(), secureService.Context, secureService, secureController, updateUsersPayload)
	// then
	require.NotNil(s.T(), result)
	// let's fetch it and validate
	_, result = test.ShowUsersOK(s.T(), nil, nil, s.controller, identity.ID.String())
	require.NotNil(s.T(), result)
	assert.Equal(s.T(), identity.ID.String(), *result.Data.ID)
	assert.Equal(s.T(), newFullName, *result.Data.Attributes.FullName)
	assert.Equal(s.T(), newImageURL, *result.Data.Attributes.ImageURL)
	assert.Equal(s.T(), newBio, *result.Data.Attributes.Bio)
	assert.Equal(s.T(), newProfileURL, *result.Data.Attributes.URL)
}

func (s *TestUsersSuite) TestUpdateUserUnauthorized() {
	// given
	user := s.createRandomUser()
	identity := s.createRandomIdentity(user, account.KeycloakIDP)
	_, result := test.ShowUsersOK(s.T(), nil, nil, s.controller, identity.ID.String())
	assert.Equal(s.T(), identity.ID.String(), *result.Data.ID)
	assert.Equal(s.T(), user.FullName, *result.Data.Attributes.FullName)
	assert.Equal(s.T(), user.ImageURL, *result.Data.Attributes.ImageURL)
	assert.Equal(s.T(), identity.ProviderType, *result.Data.Attributes.ProviderType)
	assert.Equal(s.T(), identity.Username, *result.Data.Attributes.Username)
	newEmail := "updated@email.com"
	newFullName := "newFull Name"
	newImageURL := "http://new.image.io/imageurl"
	newBio := "new bio"
	newProfileURL := "http://new.profile.url/url"
	//secureController, secureService := createSecureController(t, identity)
	updateUsersPayload := createUpdateUsersPayload(&newEmail, &newFullName, &newBio, &newImageURL, &newProfileURL)
	// when/then
	test.UpdateUsersUnauthorized(s.T(), context.Background(), nil, s.controller, updateUsersPayload)
}

func (s *TestUsersSuite) TestShowUserOK() {
	// given user
	user := s.createRandomUser()
	identity := s.createRandomIdentity(user, account.KeycloakIDP)
	// when
	_, result := test.ShowUsersOK(s.T(), nil, nil, s.controller, identity.ID.String())
	// then
	assert.Equal(s.T(), identity.ID.String(), *result.Data.ID)
	assert.Equal(s.T(), user.FullName, *result.Data.Attributes.FullName)
	assert.Equal(s.T(), user.ImageURL, *result.Data.Attributes.ImageURL)
	assert.Equal(s.T(), identity.ProviderType, *result.Data.Attributes.ProviderType)
	assert.Equal(s.T(), identity.Username, *result.Data.Attributes.Username)
}

func (s *TestUsersSuite) TestListUsersOK() {
	// given user1
	user1 := s.createRandomUser()
	identity11 := s.createRandomIdentity(user1, account.KeycloakIDP)
	identity12 := s.createRandomIdentity(user1, "github-test")
	// given user2
	user2 := s.createRandomUser()
	identity2 := s.createRandomIdentity(user2, account.KeycloakIDP)
	// when
	_, result := test.ListUsersOK(s.T(), nil, nil, s.controller)
	// then
	s.T().Log(fmt.Sprintf("User1 #%s: %s %s", user1.ID.String(), identity11.ID.String(), identity12.ID.String()))
	s.T().Log(fmt.Sprintf("User2 #%s: %s", user2.ID.String(), identity2.ID.String()))
	for i, data := range result.Data {
		s.T().Log(fmt.Sprintf("Result #%d: %s %v", i, *data.ID, *data.Attributes.Username))
	}
	assertUser(s.T(), findUser(identity11.ID, result.Data), user1, identity11)
	assertUser(s.T(), findUser(identity2.ID, result.Data), user2, identity2)
}

func (s *TestUsersSuite) createRandomUser() account.User {
	user := account.User{
		Email:    uuid.NewV4().String() + "primaryForUpdat7e@example.com",
		FullName: "A test user",
		ImageURL: "someURLForUpdate",
		ID:       uuid.NewV4(),
	}
	err := s.userRepo.Create(context.Background(), &user)
	require.Nil(s.T(), err)
	return user
}

func (s *TestUsersSuite) createRandomIdentity(user account.User, providerType string) account.Identity {
	profile := "foobarforupdate.com/" + uuid.NewV4().String() + "/" + user.ID.String()
	identity := account.Identity{
		Username:     "TestUpdateUserIntegration123" + uuid.NewV4().String(),
		ProviderType: providerType,
		ProfileURL:   &profile,
		User:         user,
		UserID:       account.NullUUID{UUID: user.ID, Valid: true},
	}
	err := s.identityRepo.Create(context.Background(), &identity)
	require.Nil(s.T(), err)
	return identity
}

func findUser(id uuid.UUID, identityData []*app.IdentityData) *app.IdentityData {
	for _, identity := range identityData {
		if *identity.ID == id.String() {
			return identity
		}
	}
	return nil
}

func assertUser(t *testing.T, actual *app.IdentityData, expectedUser account.User, expectedIdentity account.Identity) {
	require.NotNil(t, actual)
	assert.Equal(t, expectedIdentity.Username, *actual.Attributes.Username)
	assert.Equal(t, expectedIdentity.ProviderType, *actual.Attributes.ProviderType)
	assert.Equal(t, expectedUser.FullName, *actual.Attributes.FullName)
	assert.Equal(t, expectedUser.ImageURL, *actual.Attributes.ImageURL)
	assert.Equal(t, expectedUser.Email, *actual.Attributes.Email)
}

func createUpdateUsersPayload(email, fullName, bio, imageURL, profileURL *string) *app.UpdateUsersPayload {
	return &app.UpdateUsersPayload{
		Data: &app.UpdateIdentityData{
			Type: "identities",
			Attributes: &app.IdentityDataAttributes{
				Email:    email,
				FullName: fullName,
				Bio:      bio,
				ImageURL: imageURL,
				URL:      profileURL,
			},
		},
	}
}
