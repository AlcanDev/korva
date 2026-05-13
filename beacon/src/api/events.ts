import { useEffect, useRef, useState } from "react";
import { useAdminStore } from "@/stores/admin";

// Phase 8.5 — wire client for the SSE event stream.
//
// Browsers' native EventSource doesn't support custom request headers, so we
// can't pass X-Admin-Key on the SSE handshake. Two workable options:
//   1) Send the key as a query param (cheap, used here).
//   2) Use a polyfill that uses fetch + ReadableStream (heavier).
// The Vault API admin middleware already accepts `?admin_key=…` for SSE
// transport convenience — see admin auth.

export type EventKind =
	| "observation_saved"
	| "session_started"
	| "session_ended"
	| "conflict_detected"
	| "command_run"
	| "export_written"
	| "hive_phase_changed";

export interface ActivityEvent {
	kind: EventKind;
	project?: string;
	title?: string;
	actor?: string;
	meta?: Record<string, unknown>;
	at: string; // ISO timestamp
}

interface FeedState {
	events: ActivityEvent[];
	connected: boolean;
	error: string | null;
}

const MAX_EVENTS = 100;

// useActivityFeed: opens a single EventSource for the admin event stream,
// buffers the latest N events in memory, exposes connection state. Tears
// down on unmount.
export function useActivityFeed(): FeedState {
	const [events, setEvents] = useState<ActivityEvent[]>([]);
	const [connected, setConnected] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const sourceRef = useRef<EventSource | null>(null);

	useEffect(() => {
		// jsdom (Vitest) and older browsers don't have EventSource. Bail out
		// gracefully so the hook returns disconnected state instead of throwing.
		if (typeof window === "undefined" || typeof window.EventSource === "undefined") {
			setError("EventSource not supported in this environment");
			return;
		}
		const { key, sessionToken, authMode } = useAdminStore.getState();
		const auth = authMode === "session" ? sessionToken : key;
		if (!auth) {
			setError("authentication required");
			return;
		}
		// We use a URL search param because EventSource can't set custom headers.
		const url = `/vault-api/admin/events?admin_key=${encodeURIComponent(auth)}`;
		const es = new window.EventSource(url);
		sourceRef.current = es;
		es.onopen = () => {
			setConnected(true);
			setError(null);
		};
		es.onerror = () => {
			setConnected(false);
			// EventSource auto-reconnects; we don't surface intermittent
			// drops as errors, only persistent failures.
		};
		// Listen to every documented kind explicitly. addEventListener
		// matches by `event:` line in the SSE frame — we get typed events
		// without a giant switch over data.kind.
		const kinds: EventKind[] = [
			"observation_saved",
			"session_started",
			"session_ended",
			"conflict_detected",
			"command_run",
			"export_written",
			"hive_phase_changed",
		];
		const handlers: Array<[EventKind, EventListener]> = kinds.map((kind) => {
			const handler: EventListener = (e) => {
				try {
					const payload = JSON.parse((e as MessageEvent).data) as ActivityEvent;
					setEvents((prev) => {
						const next = [payload, ...prev];
						return next.length > MAX_EVENTS ? next.slice(0, MAX_EVENTS) : next;
					});
				} catch {
					// ignore malformed frames — keep stream healthy
				}
			};
			es.addEventListener(kind, handler);
			return [kind, handler];
		});
		return () => {
			for (const [kind, handler] of handlers) {
				es.removeEventListener(kind, handler);
			}
			es.close();
			sourceRef.current = null;
		};
	}, []);

	return { events, connected, error };
}
