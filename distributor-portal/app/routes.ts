import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("distributors/:distributorId/orders", "routes/distributor-orders.tsx"),
  route("distributors/:distributorId/products", "routes/distributor-products.tsx"),
] satisfies RouteConfig;
