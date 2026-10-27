# خطة M1: الواجهة (`apps/web`)

**الحالة:** مستنية موافقة · **التاريخ:** 2026-10-27

## الهدف
أول نسخة من الواجهة بتشتغل فوق السيرفر اللي اندمج في #2. الهيكل وقواعده بيتثبّتوا من دلوقتي، والشاشات نفسها هتتبني بعدين. يعني M1 فيه الـ shell والتنقّل والترجمة والاتصال بالـ API بس، ومفيش features.

## المراجع
- الـ Design System «داو» (الـ artifact الخاص): 37 component و32 أيقونة والـ tokens. [ADR-0029](ADR/0029-design-system-aurora-dusk.md).
- [ADR-0021](ADR/0021-arabic-first-ui.md): عربي أولاً، RTL، logical properties، ومفيش نصوص في الكود.
- [ADR-0022](ADR/0022-navigation-model.md): مساحة واحدة، وslide-over، و⌘K، وكل حالة ليها رابط.
- [ADR-0023](ADR/0023-ui-stack.md): React + Vite + TS + Tailwind + Radix + Motion + TanStack Query، وclient متولّد من OpenAPI.
- قواعد Sabry: `Stacks/React.md` و`Stacks/NextJS.md` (نفس الأسلوب، من غير أجزاء Next.js الخاصة بالسيرفر).

## القرارات المقترحة (محتاجة موافقة)

| # | الموضوع | الاقتراح | السبب |
|---|---|---|---|
| 1 | الهيكل | Modules زي السيرفر: `Src/Modules/<Module>/` وجواه Pages وComponents وHooks وApi وStore وTypes. وجنبهم `Src/App` و`Src/Core` و`Src/I18n` و`Src/Styles` | نفس الشكل في السيرفر والواجهة، وكل module بيتفصل لوحده. ده بيسهّل `ee/` بعدين |
| 2 | التسمية | PascalCase للفولدرات والملفات والـ components والـ state والـ handlers. والـ hooks بتبدأ بـ `use` صغيرة | `UseX` اللي في NextJS.md بتعطّل `eslint-plugin-react-hooks`، لأنه بيتعرّف على الـ hook من `use` الصغيرة. وده اللي مسك bug الـ Rollback في Deploy Center |
| 3 | الـ components | `Src/Core/Ui/` فيه الـ 37 component من الـ Design System، مكتوبين بإيدينا فوق Radix للحاجات التفاعلية (Dialog وTabs وSwitch وCheckbox وTooltip). من غير shadcn | الـ Design System بتاعنا مختلف كتير عن شكل shadcn، فنسخه وتعديله هيبقى شغل زيادة من غير فايدة |
| 4 | الحالة | TanStack Query للبيانات من السيرفر، وZustand لحالة الواجهة المشتركة، وReact Hook Form مع zod للفورمات | زي React.md بالظبط |
| 5 | التنقّل | TanStack Router، والـ slide-over مربوط بـ search param (`?panel=...`) | اتقرر قبل كده، والـ search params عنده typed |
| 6 | الـ API | `openapi-typescript` + `openapi-fetch` من `packages/openapi/openapi.yaml` نفسه، والملف المتولّد ممنوع يتعدّل | spec-first. لو السيرفر غيّر الـ envelope، الـ typecheck هيكسر فورًا |
| 7 | الترجمة | `Src/I18n/Locales/Ar.ts` و`En.ts`، والعربي هو الافتراضي، و`dir` و`lang` على `<html>` بيتغيّروا مع اللغة | ADR-0021. NextJS.md بيقول الافتراضي إنجليزي، بس قرار داو هو العربي |
| 8 | الخطوط | Poppins وIBM Plex Sans Arabic وIBM Plex Mono بـ `@fontsource`، والوزن 600 بالكتير | Self-hosted، ومفيش أي طلب لـ Google Fonts |
| 9 | التسليم | السيرفر بيخدم `dist` بـ `go:embed`. الـ Makefile بيبني الواجهة وينسخها في `apps/server/internal/App/WebDist/`، والفولدر ده في `.gitignore` ومعاه ملف placeholder | binary واحد بيتنصّب عند العميل |
| 10 | الشغل القديم | نبدأ من الأول، وناخد من worktree `m1/web` القديم الإعدادات بس (Vite وESLint وVitest وسكريبت حجم الـ bundle)، بعد ما نراجعها | الـ components القديمة مبنية على Design System اترفض (Glass وAuroraSky وخط Alexandria) |

## المهام (كل مهمة تقدر تتنفّذ لوحدها)

### 1. Scaffold
- `apps/web` بـ pnpm 10 وNode 24، وVite وReact 19 وTS strict.
- كل الاعتماديات بنسخ مثبّتة بالظبط، و`pnpm-lock.yaml` متولّد ومحدش يعدّله بإيده.
- ESLint بـ `--max-warnings 0`، وفيه react-hooks وjsx-a11y، و`max-lines` 500 و`max-lines-per-function` 100، ومنع `any` و`console`.
- Vitest مع Testing Library.
- **الملفات:** `apps/web/{package.json,vite.config.ts,tsconfig.json,eslint.config.js,index.html}`.

### 2. الـ Tokens والـ Styles
- `Src/Styles/Tokens.css` متولّد من `tokens.json` بتاع الـ Design System بسكريبت، والملف المتولّد ممنوع يتعدّل.
- `Src/Styles/Globals.css` فيه `@import "tailwindcss"` و`@theme`. مفيش `tailwind.config`، ومفيش ألوان مكتوبة في الكود.
- الـ gradients بتتعكس في RTL بالـ CSS vars، زي `bundle.css`.

### 3. `Core/Ui`
- الـ 37 component، كل واحد في ملف لوحده وليه اختبار سلوك (keyboard وfocus وaria).
- الأولوية لأجزاء الـ shell: IconRail وTopBar وButton وIconButton وSlideOver وCommandPalette وTooltip وEmptyState وBanner وSkeleton.
- الباقي هيتبني مع الشاشة اللي محتاجاه، يعني في M2.

### 4. الـ Shell (`Src/App`)
- `App.tsx` و`Router.tsx` و`Providers.tsx`.
- الـ layout: icon rail بعرض 72px، وtop bar، ومساحة شغل واحدة.
- slide-over بيتفتح من حافة الـ inline-end، ومربوط بالـ URL، والـ focus بيتحبس جواه، وEsc بيقفله.
- ⌘K وCtrl+K بيفتحوا الـ palette.
- `prefers-reduced-motion` بيوقّف الحركة.

### 5. الترجمة واللغة
- `I18n/Locales/{Ar,En}.ts` بنفس المفاتيح، واختبار بيتأكد إن المفاتيح متطابقة في الملفين.
- `useT()` للترجمة، وزرار لتغيير اللغة، والاختيار بيتحفظ في `localStorage` من غير وميض.
- `Accept-Language` بيتبعت مع كل طلب للـ API، فرسايل الأخطاء بترجع بنفس اللغة.

### 6. الـ API
- `Src/Core/Api/Schema.d.ts` متولّد، و`Client.ts` فيه `openapi-fetch` وبيفك الـ envelope.
- الأخطاء بتتحوّل لـ `TApiError` فيه `Code` و`Message`.
- `Modules/System`: `useMeta()` و`useReadiness()`. لو `SetupRequired` بـ true، بيظهر Banner بيقول إن الإعداد الأولاني لسه ماخلصش.

### 7. Module أول: `System`
- صفحة الـ workspace فيها EmptyState، وslide-over لحالة السيرفر (الـ checks والنسخة والـ commit).
- 3 حالات لكل شاشة: Loading (Skeleton)، وError (InlineAlert)، وEmpty.

### 8. التسليم من السيرفر
- `make web` بيعمل `pnpm install --frozen-lockfile` و`pnpm build`، وبعدين بينسخ الـ build لـ `App/WebDist`.
- `App/Web.go` بيخدم الملفات بـ `go:embed`، ولو المسار مش موجود بيرجّع `index.html` عشان الـ SPA.
- الـ assets ليها cache طويل وimmutable، و`index.html` بـ `no-store`.
- الـ CSP بيفضل `'self'`، ومفيش inline scripts. سكريبت الثيم بيبقى ملف لوحده.

### 9. الـ Hooks (قاعدة جديدة يبقى ليها hook واختبار)
- مكان ملفات `apps/web/Src`: `App` أو `Core/<X>` أو `I18n` أو `Styles` أو `Modules/<Name>/...`. والأسماء PascalCase، ما عدا الـ hooks (`useX.ts`) وملفات الإعدادات.
- اتجاه الـ imports: module ميعملش import لـ module تاني، و`Core` ميعملش import لـ `Modules`.
- أي نص عربي أو إنجليزي جوه JSX ممنوع، ومكانه الترجمة. الاستثناءات: `Locales/` والاختبارات.
- القواعد الحالية زي ما هي: logical properties، و`any`، و`console`، وI/T/E.

### 10. الـ CI
- job اسمه `web` بـ pnpm cache، بيشغّل `lint` و`typecheck` و`test` و`build` و`size`، ومن غير `hashFiles` على مستوى الـ job.
- `javascript-typescript` يتضاف لـ CodeQL.
- ميزانية الحجم: الـ JS الأولاني أقل من 180 KB gzip، والـ CSS أقل من 30 KB gzip.

### 11. التحقق قبل الـ push
- `pnpm lint && pnpm typecheck && pnpm test && pnpm build && pnpm size`.
- `make check` و`make build`، والسيرفر بيخدم الواجهة فعلًا.
- Playwright (موجود في البيئة): صور للـ shell بالعربي RTL والإنجليزي LTR، وتجربة فتح الـ slide-over بالـ URL، وتجربة ⌘K.

## Edge cases
- الـ API مش متاح: Banner خطأ، والـ shell فاضل شغّال.
- لغة مش مدعومة في `Accept-Language`: العربي هو اللي يشتغل.
- رابط slide-over لحاجة مش موجودة: الـ panel ميتفتحش، وبيظهر Toast.
- `localStorage` مقفول (private mode): الإعدادات الافتراضية تشتغل من غير crash.
- اسم مشروع طويل أو مكتوب بالإنجليزي جوه نص عربي: بيتعزل بـ `<bdi>`.

## مفتوح
- موافقة Sabry على القرارات من 1 لـ 10.
- branch protection على `main`.
