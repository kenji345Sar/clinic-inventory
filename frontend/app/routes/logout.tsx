import { redirect } from "react-router";
import type { Route } from "./+types/logout";
import { authDisabled, getLogoutUrl } from "../lib/auth.server";
import { destroySession, getSession } from "../lib/session.server";
import { env } from "../lib/env.server";

// /logout … セッションを破棄する。認証ONなら Auth0 側のセッションも破棄して戻す。
export async function loader({ request }: Route.LoaderArgs) {
  const session = await getSession(request.headers.get("Cookie"));
  const cookie = await destroySession(session);

  if (authDisabled) {
    return redirect("/", { headers: { "Set-Cookie": cookie } });
  }
  return redirect(getLogoutUrl(env.appBaseUrl), {
    headers: { "Set-Cookie": cookie },
  });
}

export default function Logout() {
  return null;
}
