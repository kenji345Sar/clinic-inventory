// サーバー専用の環境変数アクセス。dotenvで frontend/.env を process.env に読み込む。
// Vite は VITE_ 接頭辞のない変数を server の process.env へ自動投入しないため、明示的に読む。
import "dotenv/config";

export const env = {
  // 認証まわり
  authDisabled: process.env.AUTH_DISABLED === "true",
  auth0Domain: process.env.AUTH0_DOMAIN ?? "",
  auth0ClientId: process.env.AUTH0_CLIENT_ID ?? "",
  auth0ClientSecret: process.env.AUTH0_CLIENT_SECRET ?? "",
  auth0Audience: process.env.AUTH0_AUDIENCE ?? "",
  auth0CallbackUrl:
    process.env.AUTH0_CALLBACK_URL ?? "http://localhost:5173/auth/callback",
  // セッション署名鍵（本番は必ず .env で上書きする）
  sessionSecret: process.env.SESSION_SECRET ?? "dev-insecure-secret-change-me",
  // ログアウト後の戻り先・アプリのベースURL
  appBaseUrl: process.env.APP_BASE_URL ?? "http://localhost:5173",
} as const;
