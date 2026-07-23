import type { ReactElement } from "react";
import { render } from "@testing-library/react";
import { InvestigationUIProvider } from "@/features/alert-investigation/lib/InvestigationUIContext";

export function renderWithInvestigationUI(ui: ReactElement) {
  return render(<InvestigationUIProvider>{ui}</InvestigationUIProvider>);
}
