# CLAUDE.md — داو (Dhow)

إرشادات لأي agent بيشتغل على الـ repo ده. البشر يبدأوا من [README.md](README.md) و[CONTRIBUTING.md](CONTRIBUTING.md).

## المشروع

داو منصة Continuous Deployment آمنة بتتنصّب عند العميل. فيها control plane بـ Go، وrunners بـ Go بتشغّل الـ jobs في sandbox معزول، وواجهة React عربي أولاً.
- **الوثائق العامة** (بالإنجليزي) في `docs/`.
- **القرارات** (ADRs بالعربي) في `Plan/ADR/`. اقرا الـ ADR الخاص بأي جزء قبل ما تغيّر فيه، وأي قرار جديد يبقى ADR جديد مش تعديل في القديم.
- **السجل اليومي** في `Plan/Progress/YYYY-MM-DD.md`.

## الهيكل (ADR-0004)

| المسار | المحتوى |
|---|---|
| `apps/server` | الـ control plane (Go) |
| `apps/runner` | الـ runner (Go) |
| `apps/web` | الواجهة (React + TypeScript + Tailwind، pnpm) |
| `apps/cli` | الـ CLI `dhow` (Go) |
| `packages/openapi` | `openapi.yaml`، عقد الـ REST API (spec-first) |
| `packages/proto` | بروتوكول الـ runner مع الـ control plane (Connect/gRPC) |
| `internal/` | packages Go مشتركة بين التطبيقات |
| `ee/` | المميزات التجارية (Dhow Enterprise License)، وكل الباقي AGPL-3.0 |
| `deploy/` | Dockerfiles وCompose وHelm |
| `docs/` | وثائق عامة بالإنجليزي |
| `Plan/` | خطط وADRs وسجل يومي، بالعربي |

### هيكل `apps/server/internal` (Modular Monolith، ADR-0030)

| المجلد | الدور |
|---|---|
| `App/` | الـ router والـ middleware العامة والـ bootstrap (`Run()`)، وهو الوحيد اللي بيستورد `Modules/` |
| `Config/` | الـ env والـ validator وفتح الـ DB والـ logger والثوابت |
| `Core/<X>/` | حاجات مشتركة: `Errors` (`package apperrors`) و`Response` و`I18n` و`Middleware` و`EventBus` |
| `Database/<X>/` | `Migrations` (ملفات SQL والـ migrator) و`Sqlc` (متولّد) |
| `Modules/<Name>/` | كل ميزة لوحدها: `<Name>.Routes.go` و`.Controller.go` و`.Service.go` و`Models/` و`Queries/` و`Gen/` و`Test/{Unit,Integration,E2E,Edge}` |

- أسماء المجلدات وملفات Go PascalCase بنقط للأدوار (`System.Service.go`)، وأسماء الـ packages lowercase.
- الموديول ممنوع يستورد موديول تاني. التواصل بينهم عن طريق `Core/EventBus`. و`Core` و`Config` و`Database` ممنوع يستوردوا `Modules` أو `App`.
- كل موديول ليه `oapi-codegen.yaml` بـ `include-tags`، و`make gen` بيشغّلهم كلهم.
- أي جدول جديد فيه الأعمدة الأربعة: `"Id" uuid DEFAULT uuidv7()` و`"CreatedAt"` و`"UpdatedAt"` و`"IsDeleted"`. الاستثناء الوحيد جدول `"Instance"`.
- الحذف soft بس (ممنوع `DELETE FROM` في `Queries/`)، والوقت `timestamptz` بـ UTC، والنص `TEXT` مش `VARCHAR`، والترجمات في جدول `<Entity>Translations` مش أعمدة `NameAr`/`NameEn`.
- مسارات الـ API بـ kebab-case، وأكواد الأخطاء UPPER_SNAKE، ورسالة الخطأ بتتترجم في السيرفر من `Accept-Language`، ورسالة النجاح هي مفتاح i18n.
- PostgreSQL 18 (لـ `uuidv7()` المدمجة).

ممنوع ملفات Markdown بره `docs/` و`Plan/` و`.github/` و`.claude/`. الاستثناء: README وREADME.ar وCLAUDE وCONTRIBUTING وSECURITY وLICENSE في الـ root.

## الأوامر

- `make check`: lint وactionlint واختبارات Go بـ `-race` وgovulncheck واختبارات الـ hooks. الـ CI بيشغّل نفس الـ targets، فشغّلها قبل أي push.
- `make help` بيعرض كل الـ targets. نسخة Go مثبّتة في `go.mod`، والـ Makefile بيبني الأدوات بيها.
- الواجهة (لما `apps/web` يتعمل): `pnpm lint` و`pnpm typecheck` و`pnpm test` و`pnpm build` جوه `apps/web`.

## الـ Hooks (`.claude/settings.json`)

القواعد اللي تحت متطبّقة كـ hooks قبل أي Write أو Edit أو Bash، يعني الـ hook هيرفض أي تعديل مخالف ويقولك السبب. ده بيشمل:
- الكومنتات.
- `sh -c`.
- SQL مبني كنص.
- JSON tags مش PascalCase.
- `panic`، و`fmt.Print`، و`os.Getenv` بره الـ config.
- `any` والتسمية في TypeScript.
- الخصائص الفيزيائية (left/right) في الواجهة.
- الملفات المتولّدة.
- الأسرار.
- مكان الـ Markdown.
- طول الملف.
- الـ migrations اللي اتعملها merge.
- مكان ملفات Go في `apps/server/internal`، وتسمية PascalCase، واتجاه الاستيراد بين الطبقات.
- قواعد الـ migrations (الأعمدة الأساسية، `timestamptz`، `TEXT`، ممنوع أعمدة `Ar`/`En`) وممنوع `DELETE FROM` في الـ queries.
- مسارات `openapi.yaml` بـ kebab-case.
- رسائل الـ commits (Conventional، إنجليزي، ومن غير أي إشارة لـ AI).
- `--no-verify` والـ force-push على main.

اختباراتها في `.claude/hooks/tests/` وبتتشغّل بـ `make hooks-test`. أي قاعدة جديدة لازم يبقى ليها hook واختبار.

## قواعد ما بتتكسرش

- **ممنوع تشغيل نص جاي من المستخدم من خلال shell.** الأوامر تتبني كـ argv (`exec.CommandContext(ctx, name, args...)`)، والـ refs والمسارات يتعملها validation، و`--` قبل أي قيمة جاية من المستخدم.
- **ممنوع تقييم expressions بمفسّر عام.** شروط الـ pipeline بـ CEL بس.
- **الأسرار للكتابة بس.** ممنوع أي response أو log أو event أو إشعار يحتوي قيمة سر، ولازم اختبار يثبت ده.
- **Fail closed.** أي سر ناقص أو افتراضي، أو إعداد غلط، أو migration فشل، بيوقف الإقلاع.
- **كل endpoint بيعدّي على policy مركزية `can()`**، ومع كل route جديد صف في اختبار الـ route-table لكل دور.
- **الـ SQL من خلال sqlc بس** وبالـ parameters. ممنوع SQL مبني كنص.
- **ممنوع `panic` في كود الإنتاج، وممنوع تجاهل أي error، وممنوع global mutable state** بره `main`.
- **الاعتماديات:** تتضاف بس لما تشيل تعقيد حقيقي، وبنسخ مثبّتة بالظبط، والـ CI بيفحص كل PR.

## الأسلوب

- **الكود من غير أي كومنتات:** لا line ولا block ولا JSDoc ولا Go doc comments. الاستثناء الوحيد كومنت بيبدأ بـ `IMPORTANT:` لحاجة استثنائية فعلاً، والـ directives (`//go:`، `//nolint:`، `eslint-`...). المبرّرات مكانها `Plan/ADR`.
- **حدود الحجم:** الملف 500 سطر بالكتير، والـ function 100 سطر (`funlen` في golangci-lint، وESLint في الواجهة).
- **Go:** الأسلوب الرسمي (exported بـ PascalCase، وerrors بـ `Err` prefix)، والـ standard library الأول، و`log/slog` بـ JSON، و`context.Context` أول argument، والـ errors تتلف بـ `%w`، واختبارات table-driven.
- **الـ API:** أسماء الحقول في JSON بـ **PascalCase** (`SetupRequired`، `CreatedAt`)، والـ envelope هو `{Success, Data, Error}`. عدّل `packages/openapi/openapi.yaml` الأول، وبعدين `make gen`، وبعدين التنفيذ.
- **قاعدة البيانات:** أسماء الجداول والأعمدة بـ **PascalCase** بين علامات تنصيص (`"Projects"`، `"CreatedAt"`). أي migration بعد ما يتعمله merge ما يتعدّلش، ويتضاف واحد جديد.
- **TypeScript:**
  - الـ interfaces بـ `I` والـ types بـ `T` والـ enums بـ `E`.
  - الـ components arrow functions.
  - PascalCase للـ public methods وللـ handlers (`HandleSubmit`) ولمتغيرات الـ state (`const [Loading, setLoading]`).
  - ممنوع `any` و`console.*`.
- **الواجهة:** العربي هو اللغة الأساسية، والـ RTL هو الاتجاه الافتراضي، والـ CSS بالـ logical properties بس. الـ Design System قواعد `aurora-dark-ui` بألوان الغروب: الـ gradient عنبري ← وردي، وبيظهر بحد أقصى 3 مرات في الشاشة، والحدود edge-light، ومفيش زجاج blur ولا hover lift. والنصوص بالمصري المهذّب، والزرار بيقول هيعمل إيه بالظبط.

## الـ git

- رسائل الـ commits والـ PRs **بالإنجليزي** وبـ Conventional Commits، ومع DCO sign-off (`git commit -s`).
- **ممنوع أي إشارة لـ AI** في أي commit أو PR: لا `Co-Authored-By: Claude` ولا «Generated with Claude Code» ولا روابط sessions.
- ممنوع `--no-verify`، وممنوع force-push على `main`.

## التواصل

- مع الـ maintainer بالعربي. والكود والـ commits والوثائق العامة بالإنجليزي، والـ `Plan/` بالعربي.
- أي سؤال للـ maintainer يبقى من خلال `AskUserQuestion` بخيارات ومعاها trade-offs، ومع توصية واضحة.
- الـ subagents بـ `model: "sonnet"`.
