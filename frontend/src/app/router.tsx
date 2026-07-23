import { createBrowserRouter, Navigate } from "react-router-dom";
import { AlertInvestigationPage } from "@/features/alert-investigation/AlertInvestigationPage";
import { RootErrorBoundary } from "./RootErrorBoundary";

/**
 * Milestone 1 implements exactly one route family: Alert Investigation.
 * product-experience-brief.md §6 explicitly defers an Overview/Alerts-index
 * route until the backend list capability it depends on is separately
 * approved — this router does not build one as a placeholder.
 */
export const router = createBrowserRouter([
  {
    path: "/",
    element: <Navigate to="/alerts/1" replace />,
  },
  {
    path: "/alerts/:alertId",
    element: <AlertInvestigationPage />,
    errorElement: <RootErrorBoundary />,
  },
]);
