import { Link } from "react-router";
import type { Route } from "./+types/home";
import { api } from "../lib/api.server";

export function meta() {
  return [{ title: "卸ポータル" }];
}

export async function loader() {
  const distributors = await api.listDistributors();
  return { distributors };
}

export default function Home({ loaderData }: Route.ComponentProps) {
  const { distributors } = loaderData;
  return (
    <main className="mx-auto max-w-3xl p-8">
      <h1 className="mb-2 text-2xl font-bold">卸ポータル</h1>
      <p className="mb-6 text-sm text-gray-600">卸業者を選択してください。</p>
      <ul className="divide-y rounded border">
        {distributors.map((d) => (
          <li
            key={d.id}
            className="flex items-center justify-between p-4 hover:bg-gray-50"
          >
            <span>{d.name}</span>
            <div className="flex gap-4 text-sm">
              <Link
                to={`/distributors/${d.id}/orders`}
                className="text-blue-600 hover:underline"
              >
                受注一覧
              </Link>
              <Link
                to={`/distributors/${d.id}/products`}
                className="text-blue-600 hover:underline"
              >
                商品マスタ
              </Link>
            </div>
          </li>
        ))}
        {distributors.length === 0 && (
          <li className="p-4 text-sm text-gray-500">
            卸業者が登録されていません。
          </li>
        )}
      </ul>
    </main>
  );
}
