import { Form, Link } from "react-router";
import type { Route } from "./+types/distributor-products";
import { ApiError, api } from "../lib/api.server";
import { formatYen } from "../lib/money";

export function meta() {
  return [{ title: "商品マスタ | 卸ポータル" }];
}

export async function loader({ params }: Route.LoaderArgs) {
  const products = await api.listProducts(params.distributorId);
  return { distributorId: params.distributorId, products };
}

export async function action({ params, request }: Route.ActionArgs) {
  const form = await request.formData();
  const input = {
    distributorProductCode: String(form.get("distributorProductCode") ?? ""),
    name: String(form.get("name") ?? ""),
    vendorName: String(form.get("vendorName") ?? ""),
    vendorProductCode: String(form.get("vendorProductCode") ?? ""),
    janCode: String(form.get("janCode") ?? ""),
    unitPrice: Number(form.get("unitPrice") ?? 0),
  };

  try {
    await api.registerProduct(params.distributorId, input);
    return { ok: true as const, error: null };
  } catch (e) {
    if (e instanceof ApiError) {
      return { ok: false as const, error: e.message };
    }
    throw e;
  }
}

export default function DistributorProducts({
  loaderData,
  actionData,
}: Route.ComponentProps) {
  const { distributorId, products } = loaderData;

  return (
    <main className="mx-auto max-w-4xl p-8">
      <nav className="mb-4 text-sm">
        <Link to="/" className="text-blue-600 hover:underline">
          卸業者選択
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <span>商品マスタ</span>
      </nav>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">自社商品マスタ</h1>
        <Link
          to={`/distributors/${distributorId}/orders`}
          className="text-sm text-blue-600 hover:underline"
        >
          受注一覧へ
        </Link>
      </div>

      <section className="mb-10">
        <h2 className="mb-3 text-lg font-semibold">商品を登録</h2>
        <Form method="post" className="grid grid-cols-2 gap-3 rounded border p-4">
          <label className="text-sm">
            卸商品コード
            <input
              name="distributorProductCode"
              required
              className="mt-1 w-full rounded border p-2"
            />
          </label>
          <label className="text-sm">
            商品名
            <input name="name" required className="mt-1 w-full rounded border p-2" />
          </label>
          <label className="text-sm">
            ベンダー名
            <input
              name="vendorName"
              required
              className="mt-1 w-full rounded border p-2"
            />
          </label>
          <label className="text-sm">
            ベンダー商品コード（任意）
            <input name="vendorProductCode" className="mt-1 w-full rounded border p-2" />
          </label>
          <label className="text-sm">
            JANコード（任意）
            <input name="janCode" className="mt-1 w-full rounded border p-2" />
          </label>
          <label className="text-sm">
            標準単価（税抜・円）
            <input
              name="unitPrice"
              type="number"
              min={1}
              required
              className="mt-1 w-full rounded border p-2"
            />
          </label>
          {actionData && !actionData.ok && (
            <p className="col-span-2 text-sm text-red-600">{actionData.error}</p>
          )}
          {actionData?.ok && (
            <p className="col-span-2 text-sm text-green-700">登録しました。</p>
          )}
          <button
            type="submit"
            className="col-span-2 mt-2 rounded bg-blue-600 px-4 py-2 text-white hover:bg-blue-700"
          >
            登録
          </button>
        </Form>
      </section>

      <section>
        <h2 className="mb-3 text-lg font-semibold">登録済み商品</h2>
        {products.length === 0 ? (
          <p className="rounded border border-dashed p-4 text-sm text-gray-500">
            まだ商品が登録されていません。
          </p>
        ) : (
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b bg-gray-50 text-left">
                <th className="p-2">卸商品コード</th>
                <th className="p-2">商品名</th>
                <th className="p-2">ベンダー</th>
                <th className="p-2">JAN</th>
                <th className="p-2 text-right">標準単価</th>
                <th className="p-2">状態</th>
              </tr>
            </thead>
            <tbody>
              {products.map((p) => (
                <tr key={p.id} className="border-b">
                  <td className="p-2 font-mono">{p.distributorProductCode}</td>
                  <td className="p-2">{p.name}</td>
                  <td className="p-2">{p.vendorName}</td>
                  <td className="p-2 font-mono">{p.janCode || "—"}</td>
                  <td className="p-2 text-right">{formatYen(p.unitPrice)}</td>
                  <td className="p-2">
                    {p.discontinued ? (
                      <span className="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600">
                        廃盤
                      </span>
                    ) : (
                      <span className="rounded bg-green-100 px-2 py-1 text-xs text-green-700">
                        販売中
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </main>
  );
}
