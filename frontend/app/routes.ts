import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("facilities/:facilityId/products", "routes/facility-products.tsx"),
  route("facilities/:facilityId/orders", "routes/facility-orders.tsx"),
] satisfies RouteConfig;
