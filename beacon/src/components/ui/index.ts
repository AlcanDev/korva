// Phase 7 — Beacon UI barrel. Import every primitive from a single path:
//
//   import { Card, Button, Badge, MetricCard, Tabs } from "@/components/ui"

export { Card, CardHeader, CardBody, CardFooter } from "./Card";
export type { CardProps, CardVariant, CardTone } from "./Card";

export { Button } from "./Button";
export type { ButtonProps, ButtonVariant, ButtonSize } from "./Button";

export { Badge, StatusDot } from "./Badge";
export type { BadgeProps, BadgeTone, DotState } from "./Badge";

export { MetricCard } from "./MetricCard";
export type { MetricCardProps } from "./MetricCard";

export { Spinner, Skeleton, EmptyState, ErrorBanner } from "./Feedback";

export { PageHero } from "./PageHero";
export type { PageHeroProps } from "./PageHero";

export { Tabs } from "./Tabs";
export type { TabsProps, TabItem } from "./Tabs";
