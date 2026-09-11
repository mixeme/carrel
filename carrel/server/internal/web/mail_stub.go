package web

import "context"

// sendForgotWipeMail dispatches the wipe link (S-26). Until the mail layer
// exists it always returns false: the token stays in state, and the page must
// not claim the message was sent.
func (s *Server) sendForgotWipeMail(ctx context.Context, loginOrEmail, token string) bool {
	_ = ctx
	_ = loginOrEmail
	_ = token
	return false
}

// sendEscrowRecoveryMail dispatches recovery notice (S-26). Until the mail
// layer exists it always returns false.
func (s *Server) sendEscrowRecoveryMail(ctx context.Context, login, tempPassword string) bool {
	_ = ctx
	_ = login
	_ = tempPassword
	return false
}

// sendConfirmMail dispatches the registration confirmation link (S-26). Until
// the mail layer exists it always returns false.
func (s *Server) sendConfirmMail(ctx context.Context, login, email, token string) bool {
	_ = ctx
	_ = login
	_ = email
	_ = token
	return false
}
