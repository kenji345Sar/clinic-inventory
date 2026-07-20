package httputil

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
)

// RequireAuth は Auth0 が発行した JWT(アクセストークン)を検証するミドルウェア。
// 署名(JWKS)・発行者(issuer)・対象(audience)を検証し、失敗時は401を返す。
//
// AUTH_DISABLED=true のときは検証を丸ごと素通しする(dev バイパス)。
// Auth0テナント未準備でもローカル開発・既存の疎通確認を止めないための逃げ道。
func RequireAuth(next http.Handler) http.Handler {
	if os.Getenv("AUTH_DISABLED") == "true" {
		log.Println("[auth] AUTH_DISABLED=true: JWT検証をスキップします(dev)")
		return next
	}

	domain := os.Getenv("AUTH0_DOMAIN")
	audience := os.Getenv("AUTH0_AUDIENCE")
	if domain == "" || audience == "" {
		log.Fatal("[auth] AUTH0_DOMAIN と AUTH0_AUDIENCE が必要です(AUTH_DISABLED=true で無効化可)")
	}

	issuerURL, err := url.Parse("https://" + domain + "/")
	if err != nil {
		log.Fatalf("[auth] AUTH0_DOMAIN が不正です: %v", err)
	}

	// JWKS(公開鍵)は5分キャッシュして毎回取りに行かないようにする。
	provider := jwks.NewCachingProvider(issuerURL, 5*time.Minute)

	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{audience},
	)
	if err != nil {
		log.Fatalf("[auth] JWTバリデータの初期化に失敗: %v", err)
	}

	middleware := jwtmiddleware.New(
		jwtValidator.ValidateToken,
		jwtmiddleware.WithErrorHandler(authErrorHandler),
	)
	return middleware.CheckJWT(next)
}

// authErrorHandler は検証失敗を既存のJSONエラー形式(401)に合わせて返す。
func authErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("[auth] 認証失敗: %v", err)
	WriteJSON(w, http.StatusUnauthorized, errorResponse{Error: "認証が必要です"})
}

// ClaimsFrom は検証済みクレームをコンテキストから取り出す。
// 認可(ロール×組織階層)フェーズで、ロールや所属組織IDを読むための入口。
// AUTH_DISABLED 時はクレームが載らないため ok=false を返す。
func ClaimsFrom(ctx context.Context) (*validator.ValidatedClaims, bool) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	return claims, ok
}
