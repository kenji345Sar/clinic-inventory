import { redirect } from "react-router";
import type { Route } from "./+types/auth.callback";
import {
  decodeIdToken,
  exchangeCodeForTokens,
} from "../lib/auth.server";
import { commitSession, getSession } from "../lib/session.server";

// /auth/callback … Auth0 からの認可コードをトークンに交換し、セッションへ保存する。
export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const code = url.searchParams.get("code");
  const returnedState = url.searchParams.get("state");

  const session = await getSession(request.headers.get("Cookie"));
  const savedState = session.get("oauthState") as string | undefined;

  if (!code || !returnedState || returnedState !== savedState) {
    throw new Response("認証に失敗しました（state不一致）", { status: 400 });
  }

  const tokens = await exchangeCodeForTokens(code);
  const user = decodeIdToken(tokens.id_token);

  session.set("accessToken", tokens.access_token);
  // アクセストークン自体の有効期限。Cookieの寿命(8時間)とは別に切れるので、
  // requireAuth はこちらを見て期限切れなら再ログインさせる。
  session.set("expiresAt", Date.now() + tokens.expires_in * 1000);
  session.set("user", user);
  session.unset("oauthState");

  return redirect("/", {
    headers: { "Set-Cookie": await commitSession(session) },
  });
}

export default function AuthCallback() {
  return null;
}
