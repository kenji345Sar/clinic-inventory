// Goバックエンド（cmd/api）へのサーバーサイドAPIクライアント。
// loader/actionからのみ使う（ブラウザから直接呼ばない構成なのでCORS不要）。

const API_BASE = process.env.API_BASE ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as {
      error?: string;
    } | null;
    throw new ApiError(res.status, body?.error ?? `APIエラー (${res.status})`);
  }
  return (await res.json()) as T;
}

export interface Facility {
  id: string;
  name: string;
  facilityType: "medical" | "dental" | "vet";
  corporationId: string;
  groupId: string | null;
}

export interface Distributor {
  id: string;
  name: string;
}

export interface DistributorProduct {
  id: string;
  distributorId: string;
  distributorProductCode: string;
  name: string;
  vendorName: string;
  vendorProductCode: string;
  janCode: string;
  discontinued: boolean;
}

export interface ClinicProduct {
  id: string;
  facilityId: string;
  productCode: string;
  name: string;
  distributorProductId: string;
  janCode: string;
  reorderPoint: number;
}

export const api = {
  listFacilities: () => request<Facility[]>("/api/facilities"),
  listDistributors: () => request<Distributor[]>("/api/distributors"),
  listDistributorProducts: (distributorId: string) =>
    request<DistributorProduct[]>(`/api/distributors/${distributorId}/products`),
  listClinicProducts: (facilityId: string) =>
    request<ClinicProduct[]>(`/api/facilities/${facilityId}/products`),
  registerClinicProduct: (
    facilityId: string,
    input: {
      productCode: string;
      name?: string;
      distributorProductId: string;
      janCode?: string;
      reorderPoint: number;
    },
  ) =>
    request<ClinicProduct>(`/api/facilities/${facilityId}/products`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
};
