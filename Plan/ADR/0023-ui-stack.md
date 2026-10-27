# ADR-0023: تقنيات الواجهة: React + Tailwind + shadcn/ui + Motion

- **الحالة:** مقبول (مرحلة التصميم)
- **التاريخ:** 2026-10-27

## السياق
واجهة MUI في Deploy Center كانت تقيلة وشكلها عام. محتاجين تحكّم كامل في التصميم، وإتاحة (accessibility)، وحزم (bundles) صغيرة.

## القرار
React + Vite + TypeScript، وTailwind (بأدوات منطقية logical utilities)، وshadcn/ui فوق Radix (المكوّنات ملكنا في `packages/ui`)، وMotion، وTanStack Query، وclient مولَّد من OpenAPI. ميزانيات الأداء (performance budgets) بتتفرض في CI.

## البدائل اللي اتدرست
MUI v7؛ Mantine؛ SolidJS/Svelte.

## العواقب
إحنا اللي بنملك المكوّنات وبنصونها. دعم قوي للإتاحة وRTL.
