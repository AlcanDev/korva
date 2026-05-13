import { describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import {
	CommandPaletteProvider,
	useCommandPalette,
	useRegisterCommand,
} from "../CommandPalette";

// Phase 8.2 — verify the palette contract:
//   - ⌘K toggles open
//   - registered commands appear, fuzzy filter narrows them
//   - keyboard ↑↓ Enter Esc work
//   - clicking a command runs it and closes the modal
//   - unregister cleans up

function Registrant({
	onRun,
	id = "test-cmd",
	label = "Test command",
	keywords,
}: {
	onRun: () => void;
	id?: string;
	label?: string;
	keywords?: string[];
}) {
	useRegisterCommand({ id, label, section: "Test", run: onRun, keywords });
	return null;
}

function OpenButton() {
	const { open } = useCommandPalette();
	return (
		<button type="button" onClick={open}>
			open palette
		</button>
	);
}

describe("CommandPalette", () => {
	it("opens with ⌘K, closes with Escape", () => {
		render(
			<CommandPaletteProvider>
				<Registrant onRun={vi.fn()} />
			</CommandPaletteProvider>,
		);
		// Not open initially.
		expect(screen.queryByRole("dialog")).toBeNull();
		// ⌘K
		fireEvent.keyDown(window, { key: "k", metaKey: true });
		expect(screen.getByRole("dialog")).toBeTruthy();
		// Esc
		fireEvent.keyDown(screen.getByPlaceholderText(/type a command/i), {
			key: "Escape",
		});
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("Ctrl+K also opens the palette (non-mac platforms)", () => {
		render(
			<CommandPaletteProvider>
				<Registrant onRun={vi.fn()} />
			</CommandPaletteProvider>,
		);
		fireEvent.keyDown(window, { key: "K", ctrlKey: true });
		expect(screen.getByRole("dialog")).toBeTruthy();
	});

	it("registered commands appear in the list", () => {
		render(
			<CommandPaletteProvider>
				<Registrant onRun={vi.fn()} label="Run health check" />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		fireEvent.click(screen.getByText("open palette"));
		expect(screen.getByText("Run health check")).toBeTruthy();
	});

	it("filters by label substring", () => {
		render(
			<CommandPaletteProvider>
				<Registrant id="a" label="Apple" onRun={vi.fn()} />
				<Registrant id="b" label="Banana" onRun={vi.fn()} />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		fireEvent.click(screen.getByText("open palette"));
		const input = screen.getByPlaceholderText(/type a command/i);
		fireEvent.change(input, { target: { value: "ban" } });
		expect(screen.queryByText("Apple")).toBeNull();
		expect(screen.getByText("Banana")).toBeTruthy();
	});

	it("filters by keyword", () => {
		render(
			<CommandPaletteProvider>
				<Registrant id="c" label="Conflicts" onRun={vi.fn()} keywords={["resolve", "judgment"]} />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		fireEvent.click(screen.getByText("open palette"));
		fireEvent.change(screen.getByPlaceholderText(/type a command/i), {
			target: { value: "judgment" },
		});
		expect(screen.getByText("Conflicts")).toBeTruthy();
	});

	it("Enter runs the active command and closes", () => {
		const onRun = vi.fn();
		render(
			<CommandPaletteProvider>
				<Registrant onRun={onRun} label="Do thing" />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		fireEvent.click(screen.getByText("open palette"));
		fireEvent.keyDown(screen.getByPlaceholderText(/type a command/i), {
			key: "Enter",
		});
		expect(onRun).toHaveBeenCalledTimes(1);
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("clicking a command runs it and closes", () => {
		const onRun = vi.fn();
		render(
			<CommandPaletteProvider>
				<Registrant onRun={onRun} label="Save now" />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		fireEvent.click(screen.getByText("open palette"));
		fireEvent.click(screen.getByText("Save now"));
		expect(onRun).toHaveBeenCalledTimes(1);
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("unregister on unmount removes the command", () => {
		function Toggle({ visible }: { visible: boolean }) {
			return visible ? <Registrant onRun={vi.fn()} label="Temp" /> : null;
		}
		const view = render(
			<CommandPaletteProvider>
				<Toggle visible={true} />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		fireEvent.click(screen.getByText("open palette"));
		expect(screen.getByText("Temp")).toBeTruthy();
		// Close palette so we can unmount cleanly.
		fireEvent.keyDown(screen.getByPlaceholderText(/type/i), { key: "Escape" });
		// Unmount the Registrant.
		view.rerender(
			<CommandPaletteProvider>
				<Toggle visible={false} />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		// Re-open: the command should be gone.
		act(() => {
			fireEvent.click(screen.getByText("open palette"));
		});
		expect(screen.queryByText("Temp")).toBeNull();
	});

	it("renders empty state when no command matches the query", () => {
		render(
			<CommandPaletteProvider>
				<Registrant onRun={vi.fn()} label="Alpha" />
				<OpenButton />
			</CommandPaletteProvider>,
		);
		fireEvent.click(screen.getByText("open palette"));
		fireEvent.change(screen.getByPlaceholderText(/type/i), {
			target: { value: "zzzz" },
		});
		expect(screen.getByText(/no matches/i)).toBeTruthy();
	});
});
