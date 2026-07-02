package auth

import "context"

type contextKey string

const (
	actorContextKey   contextKey = "accounting_auth_actor"
	sessionContextKey contextKey = "accounting_auth_session"
)

// Actor contains stable authenticated identity data attached to request contexts.
type Actor struct {
	UserID string     `json:"userId"`
	Email  string     `json:"email"`
	Status UserStatus `json:"status"`
}

// ContextWithSession receives a context and session and returns a child context with auth identity data.
func ContextWithSession(ctx context.Context, session Session) context.Context {
	actor := Actor{
		UserID: session.UserID,
		Email:  session.UserEmail,
		Status: session.Status,
	}

	ctx = context.WithValue(ctx, sessionContextKey, session)
	return context.WithValue(ctx, actorContextKey, actor)
}

// ActorFromContext receives a context and returns the authenticated actor when present.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	if ctx == nil {
		return Actor{}, false
	}

	actor, ok := ctx.Value(actorContextKey).(Actor)
	return actor, ok
}

// SessionFromContext receives a context and returns the authenticated session when present.
func SessionFromContext(ctx context.Context) (Session, bool) {
	if ctx == nil {
		return Session{}, false
	}

	session, ok := ctx.Value(sessionContextKey).(Session)
	return session, ok
}
