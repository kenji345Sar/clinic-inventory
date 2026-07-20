import { useMemo, useState } from "react";
import { Form, Link, useSearchParams } from "react-router";
import type { Route } from "./+types/facility-products";
import { ApiError, api } from "../lib/api.server";
import type { DistributorProduct } from "../lib/api.server";

export function meta() {
  return [{ title: "商品マスタ | クリニック在庫管理" }];
}

// クリニック商品一覧＋卸商品検索→クリニック商品登録の画面。
// 卸商品の検索はMVPでは「選択した卸の全商品を取得してクライアント側で絞り込み」
// （卸1社あたり数千件想定。件数が問題になったらサーバーサイド検索に切り替える）。
export async function loader({ params, request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const distributorId = url.searchParams.get("distributor") ?? "";

  const [facilities, clinicProducts, distributors] = await Promise.all([
    api.listFacilities(),
    api.listClinicProducts(params.facilityId),
    api.listDistributors(),
  ]);
  const facility = facilities.find((f) => f.id === params.facilityId);
  if (!facility) {
    throw new Response("クリニックが見つかりません", { status: 404 });
  }

  const distributorProducts = distributorId
    ? await api.listDistributorProducts(distributorId)
    : [];

  return { facility, clinicProducts, distributors, distributorId, distributorProducts };
}

export async function action({ params, request }: Route.ActionArgs) {
  const form = await request.formData();
  try {
    await api.registerClinicProduct(params.facilityId, {
      productCode: String(form.get("productCode") ?? ""),
      name: String(form.get("name") ?? ""),
      distributorProductId: String(form.get("distributorProductId") ?? ""),
      janCode: String(form.get("janCode") ?? ""),
      reorderPoint: Number(form.get("reorderPoint") ?? 0),
    });
    return { ok: true as const, error: null };
  } catch (e) {
    if (e instanceof ApiError) {
      return { ok: false as const, error: e.message };
    }
    throw e;
  }
}

export default function FacilityProducts({
  loaderData,
  actionData,
}: Route.ComponentProps) {
  const { facility, clinicProducts, distributors, distributorId, distributorProducts } =
    loaderData;
  const [, setSearchParams] = useSearchParams();
  const [keyword, setKeyword] = useState("");
  const [selected, setSelected] = useState<DistributorProduct | null>(null);

  const filtered = useMemo(() => {
    if (!keyword) return distributorProducts;
    const k = keyword.toLowerCase();
    return distributorProducts.filter(
      (p) =>
        p.name.toLowerCase().includes(k) ||
        p.distributorProductCode.toLowerCase().includes(k) ||
        p.vendorName.toLowerCase().includes(k) ||
        p.janCode.includes(k),
    );
  }, [distributorProducts, keyword]);

  return (
    <main className="mx-auto max-w-5xl p-8">
      <nav className="mb-4 text-sm">
        <Link to="/" className="text-blue-600 hover:underline">
          クリニック選択
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <span>{facility.name}</span>
      </nav>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">商品マスタ — {facility.name}</h1>
        <Link
          to={`/facilities/${facility.id}/orders`}
          className="text-sm text-blue-600 hover:underline"
        >
          発注へ
        </Link>
      </div>

      <div className="grid gap-8 md:grid-cols-2">
        {/* 左: 卸商品検索 */}
        <section>
          <h2 className="mb-3 text-lg font-semibold">1. 卸商品を探す</h2>
          <label className="mb-2 block text-sm text-gray-600">
            卸業者
            <select
              className="mt-1 block w-full rounded border p-2"
              value={distributorId}
              onChange={(e) => {
                setSelected(null);
                setSearchParams(
                  e.target.value ? { distributor: e.target.value } : {},
                );
              }}
            >
              <option value="">選択してください</option>
              {distributors.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
          </label>
          {distributorId && (
            <>
              <input
                type="search"
                placeholder="商品名・卸商品コード・ベンダー・JANで絞り込み"
                className="mb-2 block w-full rounded border p-2"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
              <ul className="max-h-96 divide-y overflow-y-auto rounded border">
                {filtered.map((p) => (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => setSelected(p)}
                      disabled={p.discontinued}
                      className={`w-full p-3 text-left text-sm hover:bg-blue-50 disabled:opacity-40 ${
                        selected?.id === p.id ? "bg-blue-100" : ""
                      }`}
                    >
                      <span className="font-medium">{p.name}</span>
                      {p.discontinued && (
                        <span className="ml-2 text-xs text-red-600">廃盤</span>
                      )}
                      <br />
                      <span className="text-xs text-gray-500">
                        {p.distributorProductCode} / {p.vendorName}
                        {p.janCode && ` / JAN: ${p.janCode}`}
                      </span>
                    </button>
                  </li>
                ))}
                {filtered.length === 0 && (
                  <li className="p-3 text-sm text-gray-500">
                    該当する卸商品がありません。
                  </li>
                )}
              </ul>
            </>
          )}
        </section>

        {/* 右: クリニック商品として登録 */}
        <section>
          <h2 className="mb-3 text-lg font-semibold">
            2. クリニック商品として登録
          </h2>
          {selected ? (
            <Form method="post" className="space-y-3 rounded border p-4">
              <input
                type="hidden"
                name="distributorProductId"
                value={selected.id}
              />
              <p className="text-sm">
                <span className="text-gray-500">選択中: </span>
                {selected.name}
              </p>
              <label className="block text-sm text-gray-600">
                クリニック商品コード（必須）
                <input
                  name="productCode"
                  required
                  className="mt-1 block w-full rounded border p-2"
                  placeholder="例: C-0001"
                />
              </label>
              <label className="block text-sm text-gray-600">
                商品名（空欄なら卸商品名を引き継ぐ）
                <input
                  name="name"
                  className="mt-1 block w-full rounded border p-2"
                  placeholder={selected.name}
                />
              </label>
              <label className="block text-sm text-gray-600">
                JANコード（空欄なら卸商品から引き継ぐ）
                <input
                  name="janCode"
                  className="mt-1 block w-full rounded border p-2"
                  placeholder={selected.janCode || "なし"}
                />
              </label>
              <label className="block text-sm text-gray-600">
                発注点
                <input
                  name="reorderPoint"
                  type="number"
                  min={0}
                  defaultValue={0}
                  required
                  className="mt-1 block w-full rounded border p-2"
                />
              </label>
              {actionData && !actionData.ok && (
                <p className="text-sm text-red-600">{actionData.error}</p>
              )}
              {actionData?.ok && (
                <p className="text-sm text-green-700">登録しました。</p>
              )}
              <button
                type="submit"
                className="rounded bg-blue-600 px-4 py-2 text-white hover:bg-blue-700"
              >
                登録
              </button>
            </Form>
          ) : (
            <p className="rounded border border-dashed p-4 text-sm text-gray-500">
              左の一覧から卸商品を選択してください。
            </p>
          )}
        </section>
      </div>

      {/* 下: 登録済みクリニック商品 */}
      <section className="mt-10">
        <h2 className="mb-3 text-lg font-semibold">登録済みクリニック商品</h2>
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b bg-gray-50 text-left">
              <th className="p-2">商品コード</th>
              <th className="p-2">商品名</th>
              <th className="p-2">卸業者</th>
              <th className="p-2">JAN</th>
              <th className="p-2 text-right">発注点</th>
            </tr>
          </thead>
          <tbody>
            {clinicProducts.map((p) => (
              <tr key={p.id} className="border-b">
                <td className="p-2 font-mono">{p.productCode}</td>
                <td className="p-2">{p.name}</td>
                <td className="p-2">{p.distributorName || "—"}</td>
                <td className="p-2 font-mono">{p.janCode || "—"}</td>
                <td className="p-2 text-right">{p.reorderPoint}</td>
              </tr>
            ))}
            {clinicProducts.length === 0 && (
              <tr>
                <td colSpan={5} className="p-4 text-center text-gray-500">
                  まだ商品が登録されていません。
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </section>
    </main>
  );
}
