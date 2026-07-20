import { redirect } from "react-router";
import type { Route } from "./+types/login";
import { authDisabled, getAuthorizeUrl } from "../lib/auth.server";
import { commitSession, getSession } from "../lib/session.server";

// /login … Auth0 の認可エンドポイントへリダイレクトする。
// CSRF対策として state を生成しセッションに保存し、コールバックで突き合わせる。
export async function loader({ request }: Route.LoaderArgs) {
  if (authDisabled) {
    // dev バイパス時は既にログイン扱い。ホームへ。
    return redirect("/");
  }
  const state = crypto.randomUUID();
  const session = await getSession(request.headers.get("Cookie"));
  session.set("oauthState", state);
  return redirect(getAuthorizeUrl(state), {
    headers: { "Set-Cookie": await commitSession(session) },
  });
}

export default function Login() {
  return null;
}
