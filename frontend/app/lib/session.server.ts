import { createCookieSessionStorage } from "react-router";
import { env } from "./env.server";

// 認証情報を httpOnly Cookie に保持するセッションストレージ。
// トークンをブラウザのJSから触れない場所に置くのが目的（サーバーサイドセッション方式）。
export const sessionStorage = createCookieSessionStorage({
  cookie: {
    name: "__clinic_session",
    httpOnly: true,
    path: "/",
    sameSite: "lax",
    secrets: [env.sessionSecret],
    secure: process.env.NODE_ENV === "production",
    maxAge: 60 * 60 * 8, // 8時間
  },
});

export const { getSession, commitSession, destroySession } = sessionStorage;

// セッションに入れる値の型。
export interface SessionUser {
  sub: string;
  email?: string;
  name?: string;
}
