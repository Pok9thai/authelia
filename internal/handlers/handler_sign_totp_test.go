package handlers

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tstranex/u2f"

	"github.com/authelia/authelia/v4/internal/mocks"
	"github.com/authelia/authelia/v4/internal/models"
	"github.com/authelia/authelia/v4/internal/regulation"
	"github.com/authelia/authelia/v4/internal/session"
)

type HandlerSignTOTPSuite struct {
	suite.Suite

	mock *mocks.MockAutheliaCtx
}

func (s *HandlerSignTOTPSuite) SetupTest() {
	s.mock = mocks.NewMockAutheliaCtx(s.T())
	userSession := s.mock.Ctx.GetSession()
	userSession.Username = testUsername
	userSession.U2FChallenge = &u2f.Challenge{}
	userSession.U2FRegistration = &session.U2FRegistration{}
	err := s.mock.Ctx.SaveSession(userSession)
	require.NoError(s.T(), err)
}

func (s *HandlerSignTOTPSuite) TearDownTest() {
	s.mock.Close()
}

func (s *HandlerSignTOTPSuite) TestShouldRedirectUserToDefaultURL() {
	config := models.TOTPConfiguration{ID: 1, Username: "john", Digits: 6, Secret: []byte("secret"), Period: 30, Algorithm: "SHA1"}

	s.mock.StorageMock.EXPECT().
		LoadTOTPConfiguration(s.mock.Ctx, gomock.Any()).
		Return(&config, nil)

	s.mock.StorageMock.
		EXPECT().
		AppendAuthenticationLog(s.mock.Ctx, gomock.Eq(models.AuthenticationAttempt{
			Username:   "john",
			Successful: true,
			Banned:     false,
			Time:       s.mock.Clock.Now(),
			Type:       regulation.AuthTypeTOTP,
			RemoteIP:   models.NewNullIPFromString("0.0.0.0"),
		}))

	s.mock.TOTPMock.EXPECT().Validate(gomock.Eq("abc"), gomock.Eq(&config)).Return(true, nil)

	s.mock.Ctx.Configuration.DefaultRedirectionURL = testRedirectionURL

	bodyBytes, err := json.Marshal(signTOTPRequestBody{
		Token: "abc",
	})
	s.Require().NoError(err)
	s.mock.Ctx.Request.SetBody(bodyBytes)

	SecondFactorTOTPPost(s.mock.Ctx)
	s.mock.Assert200OK(s.T(), redirectResponse{
		Redirect: testRedirectionURL,
	})
}

func (s *HandlerSignTOTPSuite) TestShouldNotReturnRedirectURL() {
	config := models.TOTPConfiguration{ID: 1, Username: "john", Digits: 6, Secret: []byte("secret"), Period: 30, Algorithm: "SHA1"}

	s.mock.StorageMock.EXPECT().
		LoadTOTPConfiguration(s.mock.Ctx, gomock.Any()).
		Return(&config, nil)

	s.mock.StorageMock.
		EXPECT().
		AppendAuthenticationLog(s.mock.Ctx, gomock.Eq(models.AuthenticationAttempt{
			Username:   "john",
			Successful: true,
			Banned:     false,
			Time:       s.mock.Clock.Now(),
			Type:       regulation.AuthTypeTOTP,
			RemoteIP:   models.NewNullIPFromString("0.0.0.0"),
		}))

	s.mock.TOTPMock.EXPECT().Validate(gomock.Eq("abc"), gomock.Eq(&config)).Return(true, nil)

	bodyBytes, err := json.Marshal(signTOTPRequestBody{
		Token: "abc",
	})
	s.Require().NoError(err)
	s.mock.Ctx.Request.SetBody(bodyBytes)

	SecondFactorTOTPPost(s.mock.Ctx)
	s.mock.Assert200OK(s.T(), nil)
}

func (s *HandlerSignTOTPSuite) TestShouldRedirectUserToSafeTargetURL() {
	config := models.TOTPConfiguration{ID: 1, Username: "john", Digits: 6, Secret: []byte("secret"), Period: 30, Algorithm: "SHA1"}

	s.mock.StorageMock.EXPECT().
		LoadTOTPConfiguration(s.mock.Ctx, gomock.Any()).
		Return(&config, nil)

	s.mock.StorageMock.
		EXPECT().
		AppendAuthenticationLog(s.mock.Ctx, gomock.Eq(models.AuthenticationAttempt{
			Username:   "john",
			Successful: true,
			Banned:     false,
			Time:       s.mock.Clock.Now(),
			Type:       regulation.AuthTypeTOTP,
			RemoteIP:   models.NewNullIPFromString("0.0.0.0"),
		}))

	s.mock.TOTPMock.EXPECT().Validate(gomock.Eq("abc"), gomock.Eq(&config)).Return(true, nil)

	bodyBytes, err := json.Marshal(signTOTPRequestBody{
		Token:     "abc",
		TargetURL: "https://mydomain.local",
	})
	s.Require().NoError(err)
	s.mock.Ctx.Request.SetBody(bodyBytes)

	SecondFactorTOTPPost(s.mock.Ctx)
	s.mock.Assert200OK(s.T(), redirectResponse{
		Redirect: "https://mydomain.local",
	})
}

func (s *HandlerSignTOTPSuite) TestShouldNotRedirectToUnsafeURL() {
	s.mock.StorageMock.EXPECT().
		LoadTOTPConfiguration(s.mock.Ctx, gomock.Any()).
		Return(&models.TOTPConfiguration{Secret: []byte("secret")}, nil)

	s.mock.StorageMock.
		EXPECT().
		AppendAuthenticationLog(s.mock.Ctx, gomock.Eq(models.AuthenticationAttempt{
			Username:   "john",
			Successful: true,
			Banned:     false,
			Time:       s.mock.Clock.Now(),
			Type:       regulation.AuthTypeTOTP,
			RemoteIP:   models.NewNullIPFromString("0.0.0.0"),
		}))

	s.mock.TOTPMock.EXPECT().
		Validate(gomock.Eq("abc"), gomock.Eq(&models.TOTPConfiguration{Secret: []byte("secret")})).
		Return(true, nil)

	bodyBytes, err := json.Marshal(signTOTPRequestBody{
		Token:     "abc",
		TargetURL: "http://mydomain.local",
	})

	s.Require().NoError(err)
	s.mock.Ctx.Request.SetBody(bodyBytes)

	SecondFactorTOTPPost(s.mock.Ctx)
	s.mock.Assert200OK(s.T(), nil)
}

func (s *HandlerSignTOTPSuite) TestShouldRegenerateSessionForPreventingSessionFixation() {
	config := models.TOTPConfiguration{ID: 1, Username: "john", Digits: 6, Secret: []byte("secret"), Period: 30, Algorithm: "SHA1"}

	s.mock.StorageMock.EXPECT().
		LoadTOTPConfiguration(s.mock.Ctx, gomock.Any()).
		Return(&config, nil)

	s.mock.StorageMock.
		EXPECT().
		AppendAuthenticationLog(s.mock.Ctx, gomock.Eq(models.AuthenticationAttempt{
			Username:   "john",
			Successful: true,
			Banned:     false,
			Time:       s.mock.Clock.Now(),
			Type:       regulation.AuthTypeTOTP,
			RemoteIP:   models.NewNullIPFromString("0.0.0.0"),
		}))

	s.mock.TOTPMock.EXPECT().
		Validate(gomock.Eq("abc"), gomock.Eq(&config)).
		Return(true, nil)

	bodyBytes, err := json.Marshal(signTOTPRequestBody{
		Token: "abc",
	})
	s.Require().NoError(err)
	s.mock.Ctx.Request.SetBody(bodyBytes)

	r := regexp.MustCompile("^authelia_session=(.*); path=")
	res := r.FindAllStringSubmatch(string(s.mock.Ctx.Response.Header.PeekCookie("authelia_session")), -1)

	SecondFactorTOTPPost(s.mock.Ctx)
	s.mock.Assert200OK(s.T(), nil)

	s.Assert().NotEqual(
		res[0][1],
		string(s.mock.Ctx.Request.Header.Cookie("authelia_session")))
}

func TestRunHandlerSignTOTPSuite(t *testing.T) {
	suite.Run(t, new(HandlerSignTOTPSuite))
}
