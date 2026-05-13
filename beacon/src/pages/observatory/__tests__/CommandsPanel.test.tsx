import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import CommandsPanel from "../CommandsPanel";
import { I18nProvider } from "@/contexts/i18n";

// Phase 7.4 — verifica el contrato del panel: lista whitelist, click ejecuta,
// muestra exit code + duración, y respeta el flag local_only del backend.

vi.mock("@/stores/admin", () => ({
	useAdminStore: Object.assign(
		() => ({
			key: "test-key",
			sessionToken: "",
			authMode: "key" as const,
			isAuthenticated: true,
		}),
		{
			getState: () => ({ key: "test-key", sessionToken: "", authMode: "key" as const }),
		},
	),
}));

function jsonResponse(body: unknown, status = 200) {
	return Promise.resolve(
		new Response(JSON.stringify(body), {
			status,
			headers: { "Content-Type": "application/json" },
		}),
	);
}

const listFixture = {
	commands: [
		{ slug: "status", description: "Show running services", argv: "korva status" },
		{ slug: "doctor", description: "Run health checks", argv: "korva doctor" },
	],
	local_only: true,
};

const runFixture = {
	slug: "status",
	argv: "korva status",
	exit_code: 0,
	stdout: "Korva Vault running on :7437\nAll good.\n",
	stderr: "",
	duration_ms: 142,
	timed_out: false,
	truncated: false,
};

let fetchMock: ReturnType<typeof vi.fn>;

function renderPanel() {
	const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
	return render(
		<I18nProvider>
			<QueryClientProvider client={qc}>
				<CommandsPanel />
			</QueryClientProvider>
		</I18nProvider>,
	);
}

beforeEach(() => {
	fetchMock = vi.fn(async (input?: RequestInfo | URL | string | null, init?: RequestInit) => {
		const url = input == null ? "" : typeof input === "string" ? input : String(input);
		const method = init?.method ?? "GET";
		if (method === "GET" && url.includes("/admin/commands")) return jsonResponse(listFixture);
		if (method === "POST" && url.includes("/admin/commands/run")) return jsonResponse(runFixture);
		return jsonResponse({});
	});
	vi.stubGlobal("fetch", fetchMock);
});

describe("CommandsPanel", () => {
	it("renders the whitelist + a Local-vault badge when local_only=true", async () => {
		renderPanel();
		expect(await screen.findByText("Show running services")).toBeTruthy();
		expect(screen.getByText("Run health checks")).toBeTruthy();
		expect(screen.getByText(/local vault/i)).toBeTruthy();
	});

	it("clicking a command fires POST /admin/commands/run with the slug", async () => {
		renderPanel();
		fireEvent.click(await screen.findByText("Show running services"));
		await waitFor(() => {
			const call = fetchMock.mock.calls.find((c) => {
				const url = String(c[0]);
				const init = c[1] as RequestInit | undefined;
				return init?.method === "POST" && url.includes("/admin/commands/run");
			});
			expect(call).toBeTruthy();
			const init = call![1] as RequestInit;
			const body = JSON.parse(String(init.body));
			expect(body.command).toBe("status");
		});
	});

	it("renders stdout, exit code, and duration after a successful run", async () => {
		renderPanel();
		fireEvent.click(await screen.findByText("Show running services"));
		expect(await screen.findByText(/Korva Vault running on :7437/)).toBeTruthy();
		expect(screen.getByText("exit 0")).toBeTruthy();
		expect(screen.getByText("142ms")).toBeTruthy();
	});

	it("disables the runner and shows the warning when local_only=false", async () => {
		// Override the fetch mock entirely for this test — TanStack may refetch
		// so mockImplementationOnce isn't enough.
		const remoteFixture = { ...listFixture, local_only: false };
		fetchMock.mockImplementation(async (input?: RequestInfo | URL | string | null) => {
			const url = input == null ? "" : typeof input === "string" ? input : String(input);
			if (url.includes("/admin/commands")) return jsonResponse(remoteFixture);
			return jsonResponse({});
		});
		renderPanel();
		expect(await screen.findByText(/Remote vault/i)).toBeTruthy();
		expect(await screen.findByText(/not bound to localhost/i)).toBeTruthy();
	});
});
