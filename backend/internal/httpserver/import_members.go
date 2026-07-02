package httpserver

import (
	"context"
	"strings"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/auth"
	importsvc "github.com/Laisky/Accounting/backend/internal/imports"
	"github.com/Laisky/Accounting/backend/internal/ledger"
)

// resolveWacaiMemberMappings receives preview rows and user mappings, then returns row creators by source row number.
func resolveWacaiMemberMappings(ctx context.Context, authService *auth.Service, ledgerService *ledger.Service, actor ledger.Actor, actorEmail string, bookID string, rows []importsvc.PreviewRow, mappings map[string]string) (map[int]string, error) {
	normalizedMappings := normalizeWacaiMemberMappings(mappings)
	resolvedMembers := map[string]string{}
	creators := map[int]string{}

	for _, row := range rows {
		if len(row.Errors) > 0 {
			continue
		}
		rowCreator := actor.UserID
		for _, sourceName := range wacaiRowMemberSources(row) {
			sourceKey := normalizeWacaiMemberName(sourceName)
			userID, ok := resolvedMembers[sourceKey]
			if !ok {
				if isSelfWacaiMember(sourceName, actor, actorEmail) {
					userID = actor.UserID
				} else {
					var err error
					userID, err = resolveMappedWacaiMember(ctx, authService, ledgerService, actor, bookID, sourceName, normalizedMappings)
					if err != nil {
						return nil, err
					}
				}
				resolvedMembers[sourceKey] = userID
			}
			if strings.TrimSpace(row.Member) == strings.TrimSpace(sourceName) {
				rowCreator = userID
			}
		}
		creators[row.RowNumber] = rowCreator
	}

	return creators, nil
}

// normalizeWacaiMemberMappings receives raw source-name mappings and returns trimmed lookup values.
func normalizeWacaiMemberMappings(mappings map[string]string) map[string]string {
	normalized := make(map[string]string, len(mappings))
	for sourceName, userReference := range mappings {
		sourceName = normalizeWacaiMemberName(sourceName)
		userReference = strings.TrimSpace(userReference)
		if sourceName != "" && userReference != "" {
			normalized[sourceName] = userReference
		}
	}

	return normalized
}

// resolveMappedWacaiMember receives a source member name and returns the mapped existing user id.
func resolveMappedWacaiMember(ctx context.Context, authService *auth.Service, ledgerService *ledger.Service, actor ledger.Actor, bookID string, sourceName string, mappings map[string]string) (string, error) {
	sourceKey := normalizeWacaiMemberName(sourceName)
	userReference := mappings[sourceKey]
	if userReference == "" {
		return "", errors.Wrapf(ledger.ErrInvalidInput, "wacai member %q requires a uid or email mapping", sourceName)
	}

	user, err := resolveWacaiUserReference(ctx, authService, userReference)
	if err != nil {
		return "", err
	}
	if _, err := ledgerService.AddBookMember(ctx, ledger.AddBookMemberRequest{
		Actor:       actor,
		BookID:      bookID,
		UserID:      user.ID,
		Role:        ledger.RoleMember,
		DisplayName: strings.TrimSpace(sourceName),
	}); err != nil {
		return "", errors.Wrapf(err, "add mapped wacai member %q", sourceName)
	}

	return user.ID, nil
}

// resolveWacaiUserReference receives a uid or email value and returns a public active user.
func resolveWacaiUserReference(ctx context.Context, authService *auth.Service, userReference string) (auth.User, error) {
	userReference = strings.TrimSpace(userReference)
	if userReference == "" {
		return auth.User{}, errors.Wrap(ledger.ErrInvalidInput, "member mapping value is required")
	}
	if authService == nil {
		return auth.User{}, errors.Wrap(ledger.ErrInvalidInput, "auth service is required")
	}
	if strings.Contains(userReference, "@") {
		user, err := authService.ResolveUser(ctx, auth.ResolveUserRequest{Email: userReference})
		if err != nil {
			return auth.User{}, errors.Wrap(err, "resolve mapped member email")
		}
		return user, nil
	}

	user, err := authService.ResolveUser(ctx, auth.ResolveUserRequest{UserID: userReference})
	if err != nil {
		return auth.User{}, errors.Wrap(err, "resolve mapped member uid")
	}
	return user, nil
}

// wacaiRowMemberSources receives a preview row and returns unique non-empty member-like source names.
func wacaiRowMemberSources(row importsvc.PreviewRow) []string {
	sources := appendUniqueString(nil, row.Member)
	for _, participant := range row.Participants {
		sources = appendUniqueString(sources, participant)
	}

	return sources
}

// appendUniqueString receives a list and value and appends the trimmed value once.
func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.TrimSpace(existing) == value {
			return values
		}
	}

	return append(values, value)
}

// isSelfWacaiMember reports whether a source member name identifies the authenticated importer.
func isSelfWacaiMember(sourceName string, actor ledger.Actor, actorEmail string) bool {
	sourceName = normalizeWacaiMemberName(sourceName)
	switch sourceName {
	case "", "self", "me", "myself", "自己":
		return true
	default:
		return sourceName == normalizeWacaiMemberName(actor.UserID) ||
			sourceName == normalizeWacaiMemberName(actorEmail)
	}
}

// normalizeWacaiMemberName receives a source member name and returns a stable mapping key.
func normalizeWacaiMemberName(sourceName string) string {
	return strings.ToLower(strings.TrimSpace(sourceName))
}
