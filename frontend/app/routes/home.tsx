import { Link } from "react-router";
import type { Route } from "./+types/home";
import { api } from "../lib/api.server";

export function meta() {
  return [{ title: "クリニック在庫管理" }];
}

export async function loader() {
  const facilities = await api.listFacilities();
  return { facilities };
}

const facilityTypeLabel: Record<string, string> = {
  medical: "医科",
  dental: "歯科",
  vet: "獣医",
};

export default function Home({ loaderData }: Route.ComponentProps) {
  const { facilities } = loaderData;
  return (
    <main className="mx-auto max-w-3xl p-8">
      <h1 className="mb-2 text-2xl font-bold">クリニック在庫管理</h1>
      <p className="mb-6 text-sm text-gray-600">
        クリニックを選択してください。
      </p>
      <ul className="divide-y rounded border">
        {facilities.map((f) => (
          <li
            key={f.id}
            className="flex items-center justify-between p-4 hover:bg-gray-50"
          >
            <div className="flex items-center gap-3">
              <span>{f.name}</span>
              <span className="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600">
                {facilityTypeLabel[f.facilityType] ?? f.facilityType}
              </span>
            </div>
            <div className="flex gap-4 text-sm">
              <Link
                to={`/facilities/${f.id}/products`}
                className="text-blue-600 hover:underline"
              >
                商品マスタ
              </Link>
              <Link
                to={`/facilities/${f.id}/orders`}
                className="text-blue-600 hover:underline"
              >
                発注
              </Link>
            </div>
          </li>
        ))}
        {facilities.length === 0 && (
          <li className="p-4 text-sm text-gray-500">
            クリニックが登録されていません。
          </li>
        )}
      </ul>
    </main>
  );
}
