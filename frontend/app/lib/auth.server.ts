import { redirect } from "react-router";
import { env } from "./env.server";
import { getSession, type SessionUser } from "./session.server";

// AUTH_DISABLED=true のときに使う固定ユーザー（dev バイパス）。
const DEV_USER: SessionUser = {
  sub: "dev|local",
  email: "dev@example.com",
  name: "開発ユーザー",
};

export const authDisabled = env.authDisabled;

// Auth0 の認可エンドポイントURL。ログイン画面へリダイレクトさせる。
export function getAuthorizeUrl(state: string): string {
  const params = new URLSearchParams({
    response_type: "code",
    client_id: env.auth0ClientId,
    redirect_uri: env.auth0CallbackUrl,
    scope: "openid profile email",
    audience: env.auth0Audience,
    state,
  });
  return `https://${env.auth0Domain}/authorize?${params.toString()}`;
}

// 認可コードをアクセストークン・IDトークンに交換する。
export async function exchangeCodeForTokens(code: string): Promise<{
  access_token: string;
  id_token: string;
  expires_in: number;
}> {
  const res = await fetch(`https://${env.auth0Domain}/oauth/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      grant_type: "authorization_code",
      client_id: env.auth0ClientId,
      client_secret: env.auth0ClientSecret,
      code,
      redirect_uri: env.auth0CallbackUrl,
    }),
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`トークン交換に失敗しました: ${res.status} ${body}`);
  }
  return (await res.json()) as {
    access_token: string;
    id_token: string;
    expires_in: number;
  };
}

// ログアウト用URL。Auth0側のセッションも破棄し returnTo へ戻す。
export function getLogoutUrl(returnTo: string): string {
  const params = new URLSearchParams({
    client_id: env.auth0ClientId,
    returnTo,
  });
  return `https://${env.auth0Domain}/v2/logout?${params.toString()}`;
}

// IDトークン(JWT)のペイロードをデコードして表示用クレームを取り出す。
// 署名検証はしない（Auth0からTLS越しに直接受け取った直後にのみ使う前提）。
export function decodeIdToken(idToken: string): SessionUser {
  const [, payload] = idToken.split(".");
  const json = Buffer.from(payload, "base64url").toString("utf-8");
  const claims = JSON.parse(json) as {
    sub: string;
    email?: string;
    name?: string;
  };
  return { sub: claims.sub, email: claims.email, name: claims.name };
}

// loader/action の先頭で呼ぶ。未ログインなら /login へリダイレクトし、
// ログイン済みならアクセストークンとユーザー情報を返す。
// AUTH_DISABLED 時は固定ユーザーを返し、backend も dev バイパスで通す想定。
export async function requireAuth(
  request: Request,
): Promise<{ accessToken: string; user: SessionUser }> {
  if (authDisabled) {
    return { accessToken: "", user: DEV_USER };
  }
  const session = await getSession(request.headers.get("Cookie"));
  const accessToken = session.get("accessToken") as string | undefined;
  const user = session.get("user") as SessionUser | undefined;
  if (!accessToken || !user) {
    throw redirect("/login");
  }
  return { accessToken, user };
}

// 認証を強制せず、ログイン済みなら user を返す（ヘッダー表示などに使う）。
export async function getOptionalUser(
  request: Request,
): Promise<SessionUser | null> {
  if (authDisabled) {
    return DEV_USER;
  }
  const session = await getSession(request.headers.get("Cookie"));
  return (session.get("user") as SessionUser | undefined) ?? null;
}
