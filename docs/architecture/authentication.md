# 認証(Auth0)の仕組みと運用

このドキュメントは、本システムの認証をなぜ・どう構成したか、および今後どうユーザーを運用するかをまとめる。

最終更新: 2026-07-20

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

---

## 3. ログインの流れ(毎回)

1. 未ログインでアプリを開くと `/login` にリダイレクト
2. `/login` が Auth0 のログイン画面へ送る
3. 利用者が Auth0 でログイン(またはサインアップ)
4. Auth0 が `/auth/callback` にコード付きで戻す
5. アプリのサーバがコードをトークンに交換し、**httpOnly Cookie のセッション**に保存
6. 以後、アプリ→バックエンドの通信にトークンを添付し、バックエンドが検証して応答

トークンはブラウザのJavaScriptから触れない場所(httpOnly Cookie)に置くため、盗まれにくい。

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

## 6. よくある疑問

- **Q. ユーザーごとにアプリのDBにも登録が要る?**
  A. 現状は不要。ユーザーはAuth0が持ち、アプリはトークンで本人を知る。
  認可を入れる段階で「このAuth0ユーザーはこのクリニックのスタッフ」という対応情報が要るが、
  それも Auth0 側の属性(クレーム)で持たせる方針で、必ずしもアプリDBにユーザー表を作るとは限らない。

- **Q. パスワードを忘れたら?**
  A. ログイン画面の「Reset password」から Auth0 がリセットする。アプリ側の対応は不要。

- **Q. 本番公開時に変えるものは?**
  A. Callback/Logout URL を本番ドメインに追加、`SESSION_SECRET` を本番用のランダム値に、
  `.env` の値を本番環境の環境変数に設定。テナントは開発用(dev-)とは別に本番用を用意するのが望ましい。
