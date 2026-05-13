import { useNavigate } from "react-router";
import {
	LayoutDashboard,
	Telescope,
	FolderTree,
	Download,
	Terminal,
	GitMerge,
	Settings,
	Database,
	Clock,
	BookOpen,
	Shield,
	KeyRound,
} from "lucide-react";
import { useRegisterCommand } from "@/components/ui";

// Phase 8.2 — Top-level navigation commands. Registered once via App so
// they're always discoverable via ⌘K from any page.
//
// Pages can register page-specific commands of their own with the same
// useRegisterCommand hook (e.g. "Refresh projects list", "Run dry-run
// prune"). The registry merges everything alphabetically inside each
// section.

export default function GlobalCommands() {
	const navigate = useNavigate();

	useRegisterCommand({
		id: "nav-dashboard",
		section: "Navigation",
		label: "Go to dashboard",
		icon: <LayoutDashboard size={14} />,
		keywords: ["home", "overview", "main"],
		run: () => navigate("/admin/dashboard"),
	});

	useRegisterCommand({
		id: "nav-observatory-health",
		section: "Navigation",
		label: "Go to System health",
		icon: <Telescope size={14} />,
		keywords: ["observatory", "status", "health"],
		run: () => navigate("/admin/observatory/health"),
	});

	useRegisterCommand({
		id: "nav-commands",
		section: "Navigation",
		label: "Go to Commands",
		icon: <Terminal size={14} />,
		hint: "Run korva CLI",
		keywords: ["terminal", "korva", "cli", "run"],
		run: () => navigate("/admin/observatory/commands"),
	});

	useRegisterCommand({
		id: "nav-projects",
		section: "Navigation",
		label: "Go to Projects",
		icon: <FolderTree size={14} />,
		hint: "Inventory + consolidate + prune",
		keywords: ["consolidate", "prune", "merge"],
		run: () => navigate("/admin/observatory/projects"),
	});

	useRegisterCommand({
		id: "nav-export",
		section: "Navigation",
		label: "Go to Obsidian export",
		icon: <Download size={14} />,
		keywords: ["obsidian", "markdown", "wikilinks"],
		run: () => navigate("/admin/observatory/export"),
	});

	useRegisterCommand({
		id: "nav-conflicts",
		section: "Navigation",
		label: "Go to Conflicts",
		icon: <GitMerge size={14} />,
		keywords: ["judgment", "resolve"],
		run: () => navigate("/admin/observatory/conflicts"),
	});

	useRegisterCommand({
		id: "nav-config",
		section: "Navigation",
		label: "Go to Configuration",
		icon: <Settings size={14} />,
		keywords: ["config", "settings", "korva.config.json"],
		run: () => navigate("/admin/observatory/config"),
	});

	useRegisterCommand({
		id: "nav-vault",
		section: "Navigation",
		label: "Open vault browser",
		icon: <Database size={14} />,
		keywords: ["observations", "search", "browse"],
		run: () => navigate("/admin/vault"),
	});

	useRegisterCommand({
		id: "nav-sessions",
		section: "Navigation",
		label: "View sessions",
		icon: <Clock size={14} />,
		run: () => navigate("/admin/sessions"),
	});

	useRegisterCommand({
		id: "nav-scrolls",
		section: "Navigation",
		label: "Open scrolls",
		icon: <BookOpen size={14} />,
		keywords: ["lore", "knowledge"],
		run: () => navigate("/admin/scrolls"),
	});

	useRegisterCommand({
		id: "nav-license",
		section: "Navigation",
		label: "License status",
		icon: <KeyRound size={14} />,
		run: () => navigate("/admin/license"),
	});

	useRegisterCommand({
		id: "nav-sentinel",
		section: "Navigation",
		label: "Sentinel rules",
		icon: <Shield size={14} />,
		keywords: ["lint", "rules", "guardrails"],
		run: () => navigate("/admin/observatory/sentinel"),
	});

	return null;
}
