package httputil

import (
	"context"
	"errors"
	"testing"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/validator"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// contextWithAppClaims はミドルウェアを経由せず、検証済みクレームが載った状態の
// コンテキストを直接組み立てる(AuthorizeFacilityの判定ロジックだけをテストするため)。
func contextWithAppClaims(claims *AppClaims) context.Context {
	validated := &validator.ValidatedClaims{CustomClaims: claims}
	return context.WithValue(context.Background(), jwtmiddleware.ContextKey{}, validated)
}

func TestAuthorizeFacility(t *testing.T) {
	const facilityA = "38057321-88d6-4a6c-8820-0da36fdd3766"
	const facilityB = "38057321-88d6-4a6c-8820-000000000000"

	tests := []struct {
		name       string
		ctx        context.Context
		facilityID string
		wantErr    bool
	}{
		{
			name:       "クレームが無い(AUTH_DISABLED相当)は許可",
			ctx:        context.Background(),
			facilityID: facilityA,
			wantErr:    false,
		},
		{
			name:       "roleが未設定(移行期のユーザー)は許可",
			ctx:        contextWithAppClaims(&AppClaims{}),
			facilityID: facilityA,
			wantErr:    false,
		},
		{
			name:       "adminは全クリニックを許可",
			ctx:        contextWithAppClaims(&AppClaims{Role: RoleAdmin}),
			facilityID: facilityA,
			wantErr:    false,
		},
		{
			name:       "facility_userは自分のクリニックのみ許可",
			ctx:        contextWithAppClaims(&AppClaims{Role: RoleFacilityUser, FacilityID: facilityA}),
			facilityID: facilityA,
			wantErr:    false,
		},
		{
			name:       "facility_userは他クリニックへのアクセスを拒否",
			ctx:        contextWithAppClaims(&AppClaims{Role: RoleFacilityUser, FacilityID: facilityA}),
			facilityID: facilityB,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AuthorizeFacility(tt.ctx, tt.facilityID)
			if tt.wantErr {
				if !errors.Is(err, shareddomain.ErrForbidden) {
					t.Fatalf("expected ErrForbidden, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
