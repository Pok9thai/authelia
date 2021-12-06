package handlers

import (
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/regulation"
)

// SecondFactorTOTPPost validate the TOTP passcode provided by the user.
func SecondFactorTOTPPost(ctx *middlewares.AutheliaCtx) {
	requestBody := signTOTPRequestBody{}

	if err := ctx.ParseBody(&requestBody); err != nil {
		ctx.Logger.Errorf(logFmtErrParseRequestBody, regulation.AuthTypeTOTP, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	userSession := ctx.GetSession()

	config, err := ctx.Providers.StorageProvider.LoadTOTPConfiguration(ctx, userSession.Username)
	if err != nil {
		ctx.Logger.Errorf("Failed to load TOTP configuration: %+v", err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	isValid, err := ctx.Providers.TOTP.Validate(requestBody.Token, config)
	if err != nil {
		ctx.Logger.Errorf("Failed to perform TOTP verification: %+v", err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if !isValid {
		_ = markAuthenticationAttempt(ctx, false, nil, userSession.Username, regulation.AuthTypeTOTP, nil)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if err = markAuthenticationAttempt(ctx, true, nil, userSession.Username, regulation.AuthTypeTOTP, nil); err != nil {
		respondUnauthorized(ctx, messageMFAValidationFailed)
		return
	}

	if err = ctx.Providers.SessionProvider.RegenerateSession(ctx.RequestCtx); err != nil {
		ctx.Logger.Errorf(logFmtErrSessionRegenerate, regulation.AuthTypeTOTP, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	userSession.SetTwoFactor(ctx.Clock.Now())

	if err = ctx.SaveSession(userSession); err != nil {
		ctx.Logger.Errorf(logFmtErrSessionSave, "authentication time", regulation.AuthTypeTOTP, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if userSession.OIDCWorkflowSession != nil {
		handleOIDCWorkflowResponse(ctx)
	} else {
		Handle2FAResponse(ctx, requestBody.TargetURL)
	}
}
