# 認証(Auth0)の仕組みと運用

このドキュメントは、本システムの認証をなぜ・どう構成したか、および今後どうユーザーを運用するかをまとめる。

最終更新: 2026-09-08

---

## 1. 基本の考え方

**ユーザーアカウントの実体は Auth0 が持つ。アプリ(このリポジトリ)はユーザーテーブルを持たない。**

- ログイン画面・パスワード管理・本人確認は Auth0 が担当する。
- アプリは「Auth0 が発行した本物のトークンか」を検証し、正しければ本人とみなす。
- そのため、パスワードを自前で保存・管理しない(漏洩リスク・実装コストを Auth0 に寄せる)。

```
利用者 ──ログイン──▶ Auth0(ID・パスワードを管理)
                        │ 本人確認OKならトークンを発行
                        ▼
利用者のブラウザ ──トークン──▶ このアプリ(トークンを検証して信用)
```

---

## 2. 今回設定した流れ(2026-07-20 に実施済み)

Auth0ダッシュボードで一度だけ行った初期設定:

1. **テナント作成** … `dev-iz8dclh4osh86hd1.us.auth0.com`(このプロジェクト専用のAuth0領域)
2. **Application 登録**(名前: My App / 種別: Regular Web App)
   - ログイン後の戻り先(Callback): `http://localhost:5173/auth/callback`
   - ログアウト後の戻り先: `http://localhost:5173`
   - ここで発行される `Client ID` / `Client Secret` をアプリが使う
3. **API 登録**(名前: clinic-inventory-api / 識別子(audience): `https://api.clinic-inventory`)
   - バックエンドはこの audience 宛のトークンだけを受け付ける
4. **Application → API のアクセス許可**(API の「アプリケーションアクセス」で My App を許可)
   - これが無いと「Client is not authorized to access ...」エラーになる
5. **アプリ側の設定** … `frontend/.env` に上記の値を設定し `AUTH_DISABLED=false`

これらは**初回だけ**の作業。以後ユーザーが増えても再設定は不要。

### 管理画面(Auth0ダッシュボード)への入り方

設定を変更するには https://manage.auth0.com/ にログインし、テナント`dev-iz8dclh4osh86hd1`を開く。
入り方を忘れると設定変更が一切できなくなるため、手段をここに記録しておく。

| 項目 | 内容 |
|---|---|
| ログイン方法 | **Googleログイン**(`管理者のGoogleアカウント`)。メール+パスワードは設定していないため、パスワード欄に何を入れても通らない |
| MFA(2要素認証) | **必須**。Auth0側の仕様でスキップできない。Google Authenticatorに`Auth0: 管理者のGoogleアカウント`として登録済み(2026-08-23) |
| 毎回の手順 | 「Googleで続ける」→ Google Authenticatorの6桁コードを入力。**QRコードの読み取りは初回だけ**で、次回以降は不要 |
| リカバリコード | スマホを失くした場合の最後の手段。保管場所は各自で管理する(**このリポジトリには置かない**) |

注意: ここで言う`管理者のGoogleアカウント`は**テナントの持ち主(管理者)のアカウント**で、
4章(A)に出てくる同じアドレスの**アプリ利用者アカウントとは別物**。
たまたま同じメールアドレスを使っているだけで、前者はAuth0の設定を変えられ、後者はアプリにログインできるだけ。

---

## 3. ログインの流れ(毎回) — ソースを辿る

先に結論から言うと、**アプリのコードがAuth0へHTTPを投げている箇所は2つしかない**。

| Auth0への通信 | コード |
|---|---|
| `POST /oauth/token`(認可コード→トークンの交換) | [auth.server.ts](../../frontend/app/lib/auth.server.ts) `exchangeCodeForTokens` の `fetch` |
| JWKS(署名検証用の公開鍵)の取得 | [auth.go](../../backend/internal/handler/httputil/auth.go) `jwks.NewCachingProvider` |

残りの `getAuthorizeUrl` / `getLogoutUrl` は**URL文字列を組み立てて `redirect()` に渡すだけ**で、通信するのはブラウザ。
この区別を押さえたうえで、関数の呼び出し順に追う。

### 3-1. 入口 — `home.tsx` の loader

画面を開くと最初に動くのはコンポーネントではなく loader。その1行目が `requireAuth()`。

```ts
// frontend/app/routes/home.tsx
export async function loader({ request }: Route.LoaderArgs) {
  const { accessToken, user } = await requireAuth(request);   // ← ここで止まるか進むかが決まる
  const facilities = await api.listFacilities(accessToken);
  return { facilities, user };
}
```

`requireAuth` はCookieのセッションを覗き、トークンが無ければ**値を返さず `throw` する**。

```ts
// frontend/app/lib/auth.server.ts
if (!accessToken || !user || !expiresAt || Date.now() >= expiresAt - 60_000) {
  throw redirect("/login");
}
```

`return` ではなく `throw` なので、呼び出し側の loader は以降を実行しない。
「未ログインならログイン画面へ」が各画面のif文ではなく**この1関数に集約**されているのはこの `throw` のため。

### 3-2. `/login` — Auth0へ渡すURLを組み立てるだけ

```ts
// frontend/app/routes/login.tsx
const state = crypto.randomUUID();
session.set("oauthState", state);          // 後で照合するために自分のCookieへ控える
return redirect(getAuthorizeUrl(state), {
  headers: { "Set-Cookie": await commitSession(session) },
});
```

`getAuthorizeUrl` の中身は `URLSearchParams` の組み立てと文字列結合だけで、`fetch` は無い。

```ts
// frontend/app/lib/auth.server.ts
return `https://${env.auth0Domain}/authorize?${params.toString()}`;
```

ここで載せる `audience` が、**バックエンドが受け付ける相手**(`https://api.clinic-inventory`)を指定している。
これを外すとAuth0はAPI用ではないトークンを返し、Go側の検証で401になる。

### 3-3. `/auth/callback` — 唯一Auth0へ`fetch`する場所

Auth0がブラウザを `?code=…&state=…` 付きで戻してくる。受け口の順序は「照合 → 交換」。

```ts
// frontend/app/routes/auth.callback.tsx
const savedState = session.get("oauthState");
if (!code || !returnedState || returnedState !== savedState) {
  throw new Response("認証に失敗しました（state不一致）", { status: 400 });
}
const tokens = await exchangeCodeForTokens(code);
```

`exchangeCodeForTokens` が実際の通信。ここだけ `client_secret` を使う。

```ts
// frontend/app/lib/auth.server.ts
const res = await fetch(`https://${env.auth0Domain}/oauth/token`, {
  method: "POST",
  body: JSON.stringify({
    grant_type: "authorization_code",
    client_id: env.auth0ClientId,
    client_secret: env.auth0ClientSecret,   // ← ブラウザには絶対に出せない値
    code,
    redirect_uri: env.auth0CallbackUrl,
  }),
});
```

このファイルが `.server.ts` なのは飾りではなく、**バンドルされてブラウザへ配られないことの保証**。
`code` を「引換券」として一度ブラウザ経由で受け取り、秘密を使う交換だけをサーバー間で行うのが認可コードフローの要点。

### 3-4. セッションへ載せる

```ts
// frontend/app/routes/auth.callback.tsx
session.set("accessToken", tokens.access_token);
session.set("expiresAt", Date.now() + tokens.expires_in * 1000);
session.set("user", decodeIdToken(tokens.id_token));
session.unset("oauthState");               // 使い終わったstateは残さない
return redirect("/", { headers: { "Set-Cookie": await commitSession(session) } });
```

`decodeIdToken` は **署名検証をせず** Base64をデコードしているだけ。
Auth0からTLS越しに直接受け取った直後の値にしか使わないため([auth.server.ts](../../frontend/app/lib/auth.server.ts)のコメント)。
画面右上に出るメールアドレスはここで取り出した `email` クレームで、DBのユーザー表由来ではない。

Cookieの実体は [session.server.ts](../../frontend/app/lib/session.server.ts) の `createCookieSessionStorage`。
`httpOnly: true` でブラウザのJSから読めず、`secrets: [env.sessionSecret]` で署名される。
`maxAge` は8時間だが、**アクセストークン自体の寿命はAuth0側の設定で別に切れる**。
両方を見る必要があるので、3-1の判定は `expiresAt` を持っている。

### 3-5. 2周目の `home.tsx` — 今度は通る

`requireAuth` がトークンを返し、`api.listFacilities(accessToken)` へ進む。

```ts
// frontend/app/lib/api.server.ts
const authHeader = accessToken ? { Authorization: `Bearer ${accessToken}` } : {};
const res = await fetch(`${API_BASE}${path}`, { ...init, headers: { ...authHeader, ... } });
```

呼んでいるのはブラウザではなく**フロントのNodeプロセス**。
そのためトークンがブラウザのJSに渡らず、ブラウザ→Goの直通が無いのでCORS設定も要らない。

### 3-6. Go側 — Auth0への通信はJWKS取得だけ

ルーティングの1行で `/api/` 配下がまとめてミドルウェアを通る。

```go
// backend/cmd/api/main.go
root.Handle("/api/portal/", portal)                        // 卸ポータルは素通し(6章)
root.Handle("/api/", httputil.RequireAuth(protected))
```

`RequireAuth` の中では、公開鍵を取りに行くプロバイダを**1回だけ**作る。

```go
// backend/internal/handler/httputil/auth.go
provider := jwks.NewCachingProvider(issuerURL, 5*time.Minute)
jwtValidator, err := validator.New(
    provider.KeyFunc, validator.RS256, issuerURL.String(), []string{audience},
    validator.WithCustomClaims(func() validator.CustomClaims { return &AppClaims{} }),
    validator.WithAllowedClockSkew(time.Minute),
)
return middleware.CheckJWT(next)
```

`RequireAuth` は `main()` の組み立て時に一度呼ばれ、返ってきた `http.Handler` が常駐する。
**リクエストごとに実行されるのは `CheckJWT` の内側だけ**で、そこはAuth0へ問い合わせない。
公開鍵で署名を検証し、`issuer` と `audience` を突き合わせるローカル計算で完結する
(だからAPIを何回叩いてもAuth0への通信は増えない)。

検証を通ったクレームは `context` に載り、ハンドラは `AppClaimsFrom(r.Context())` で読む。

### 3-7. 一覧が返り、画面になる

```go
// backend/internal/handler/organization/handler.go
facilities, err := h.facilityRepo.FindAll(r.Context())
if claims, ok := httputil.AppClaimsFrom(r.Context()); ok && claims.Role == httputil.RoleFacilityUser {
    // 自分の所属クリニックだけに絞る
}
```

現在ログイン中のユーザーには `role` を割り当てていないため `ok && ...` が偽になり、全件が返る(5章)。
`facility_user` を設定すれば、この分岐で**一覧そのものが1件になる**。
ここから先(repository → gorm → SQL)は [request-to-sql-flow.md 1章](../go/request-to-sql-flow.md#1-読み取りの1往復--クリニック一覧)。

### 3-8. 補足

- `AUTH_DISABLED=true` のときは 3-2〜3-4 が丸ごと動かない。フロントの `requireAuth` は固定の `DEV_USER` を返し、Goの `RequireAuth` は `next` をそのまま返して検証器を組み立てない。
- ログアウトは [logout.tsx](../../frontend/app/routes/logout.tsx) が `destroySession` でCookieを捨て、`getLogoutUrl` でAuth0の `/v2/logout` へ送る。アプリ側だけ捨ててもAuth0にセッションが残り、次回ログイン画面が出ずに素通りしてしまう。

---

## 4. 今後の運用: ユーザーをどう増やすか

**「ユーザーを追加」= Auth0 にアカウントを用意すること。** 方法は3通りあり、用途で選ぶ。

### (A) セルフサインアップ(利用者が自分で登録)
- ログイン画面の「Sign up」から利用者自身がメール+パスワードで登録。
- 一番手間がかからない。今回 `管理者のGoogleアカウント` はこの方法で登録された。
- 誰でも登録できてしまうと困る場合は、後述の「サインアップ無効化」で管理者作成のみに絞れる。

### (B) 管理者が作成(Auth0ダッシュボード)
- **User Management → Users → Create User** で1人ずつ作成(メール・初期パスワードを設定)。
- 少人数を管理者側でコントロールしたいときに向く。
- 一括登録は **Import Users**(CSV/JSON)で可能。

### (C) ソーシャルログイン(Google等)
- 「Continue with Google」で Google アカウントでそのままログイン。
- アカウント作成の手間が無い。社内でGoogle Workspaceを使っているなら便利。

> つまり「1人ずつ手作業で登録」だけではない。セルフサインアップやソーシャルを使えば、
> 管理者が1件ずつ作る必要はない。運用方針(誰でも登録可か、管理者承認制か)で選ぶ。

### サインアップを閉じたい場合(招待制にしたい)
- Auth0 の該当 Database 接続の設定で **Disable Sign Ups** をオンにすると、
  セルフサインアップを止め、(B)の管理者作成 or 招待のみに絞れる。

---

## 5. 「ログインできること」と「何を見られるか」は別物(重要)

今: ログインさえできれば全クリニックが見える。クリニックごとに絞り込む仕組みはコードとしては用意した([auth.go](../../backend/internal/handler/httputil/auth.go))が、Auth0側で「このユーザーはこのクリニック」と設定するまでは効かない。

理由: ログインアカウントは本人がセルフサインアップできても、「この人はどのクリニックの担当か」は本人の自己申告に任せられない(なりすませてしまう)。だから管理者がAuth0側で1人ずつ割り当てる必要があり、その作業がまだ済んでいない。

### 有効にする手順(Auth0ダッシュボード、3ステップ)

管理画面は https://manage.auth0.com/ からログインし、テナント(`dev-iz8dclh4osh86hd1`)を選ぶ。

1. **User Management → Users** で対象ユーザーの**App Metadata**に設定する。
   ```json
   { "role": "facility_user", "facility_id": "38057321-88d6-4a6c-8820-0da36fdd3766" }
   ```
   管理者にする場合は `{ "role": "admin" }` だけでよい(全クリニックが見える)。
2. **Actions → Flows → Login** にPost-Login Actionを追加し、上記の値をアクセストークンのカスタムクレームへ転記する。
   ```js
   exports.onExecutePostLogin = async (event, api) => {
     const { role, facility_id } = event.user.app_metadata;
     if (role) api.accessToken.setCustomClaim("https://api.clinic-inventory/role", role);
     if (facility_id) api.accessToken.setCustomClaim("https://api.clinic-inventory/facility_id", facility_id);
   };
   ```
3. 対象ユーザーで再ログインする(既存のログインセッションにはクレームが載っていないため)。

これをやるまでは今まで通り「全部見える」で動く。

---

## 6. 卸ポータルの認証(未実装)

卸ポータル(`distributor-portal`、`localhost:5174`)は**ログイン機能を持たない**。
アプリ自体は動いているが、開くとそのまま画面が出る。
バックエンドの`/api/portal/...`も`RequireAuth`を通しておらず、トークン無しでアクセスできる
([main.go](../../backend/cmd/api/main.go)のルーティング)。卸業者にAuth0アカウントを用意していないため、
現時点では意図的にそうしている。

これは**デプロイした時点で問題になる**。卸IDはURLパラメータなので、URLさえ分かれば
他社の受注一覧や医院別単価(=他院との取引条件)が見える。ローカル開発中は実害が無いだけ。

### 認証を入れるときにやること

| # | 作業 | どちらの作業か |
|---|---|---|
| 1 | Auth0に**Applicationをもう1つ登録**する | Auth0ダッシュボード |
| 2 | 卸ポータルにログイン画面とコールバックを作る | コード |
| 3 | `/api/portal/...`を`RequireAuth`の内側へ移す | コード |
| 4 | `AuthorizeDistributor`(卸版の権限判定)を作る | コード |
| 5 | 各ユーザーのapp_metadataに卸の所属を書く | Auth0ダッシュボード |

**テナントは分けなくてよい。** 1テナントの中にApplicationを2つ置き、ユーザーは共通のまま
`app_metadata`で「クリニックの人か卸の人か」を区別する(5章と同じ仕組み)。

1で登録する内容:

```
アプリケーション → アプリケーションを作成
  名前:          卸ポータル
  種別:          Regular Web App
  Callback URL:  http://localhost:5174/auth/callback
  Logout URL:    http://localhost:5174
```

発行された`Client ID` / `Client Secret`を`distributor-portal/.env`に設定する
(クリニック向けフロントの`frontend/.env`と同じ形)。

2は既存の`frontend/app/routes/login.tsx`・`auth.callback.tsx`・`lib/auth.server.ts`を
雛形にできるため、ゼロからの実装にはならない。

### 権限の粒度

卸業者担当者は「自社と契約しているクリニックのデータのみ閲覧、更新は不可」というルール
([domain-rules.md](domain-rules.md)の卸ポータルコンテキスト)。ただし**契約を表すデータが未実装**のため、
1〜5を全て終えても「自社のデータだけ見せる」までしか絞れず、「契約しているクリニックだけ」は
実現できない([requirements.md 未確定事項](../requirements.md#未確定事項))。

なお現状、卸ポータルのAPIには`POST /api/portal/distributors/{id}/products`(卸商品の登録)が生えており、
「閲覧のみ」というルールと矛盾している。認証を入れる際にこのエンドポイントの要否も判断する。

---

## 7. よくある疑問

- **Q. ユーザーごとにアプリのDBにも登録が要る?**
  A. 現状は不要。ユーザーはAuth0が持ち、アプリはトークンで本人を知る。
  認可を入れる段階で「このAuth0ユーザーはこのクリニックのスタッフ」という対応情報が要るが、
  それも Auth0 側の属性(クレーム)で持たせる方針で、必ずしもアプリDBにユーザー表を作るとは限らない。

- **Q. パスワードを忘れたら?**
  A. ログイン画面の「Reset password」から Auth0 がリセットする。アプリ側の対応は不要。

- **Q. 本番公開時に変えるものは?**
  A. Callback/Logout URL を本番ドメインに追加、`SESSION_SECRET` を本番用のランダム値に、
  `.env` の値を本番環境の環境変数に設定。テナントは開発用(dev-)とは別に本番用を用意するのが望ましい。
