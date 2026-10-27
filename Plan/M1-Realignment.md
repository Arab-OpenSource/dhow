# خطة تعديل M1 على قواعد Sabry

**الحالة:** مستنية موافقة · **التاريخ:** 2026-10-27

## الخلفية

أول جزء من M1 (PR #1) اتبنى قبل ما ألاقي `sabry-claude-kit`، فطلع مخالف لكذا قاعدة. الخطة دي بتظبط كل اللي اتعمل وبتحدد إزاي هيتكمّل.

## القرارات اللي Sabry خدها (2026-10-27)

| الموضوع | القرار |
|---|---|
| تسمية JSON في الـ API | PascalCase (`SetupRequired`, `CreatedAt`)، والـ response envelope هو `{Success, Data, Error}` زي `Stacks/Go.md` |
| أسماء الجداول والأعمدة في PostgreSQL | PascalCase بين علامات تنصيص (`"Instance"`, `"CreatedAt"`) |
| كود Go | أسلوب Go الرسمي، وحد أقصى 500 سطر للملف و100 سطر للـ function، والـ lint بيفرضهم |
| تسمية TypeScript | من `Stacks/React.md` وقواعد Deploy Center: `I`/`T`/`E` prefixes، وPascalCase للـ public methods والـ handlers والـ state، وarrow components |
| الإشارة لـ AI في الـ git | من دلوقتي ورايح بس، من غير إعادة كتابة التاريخ |
| صاحب الـ commits | `Sabry Dawood <kazouya25@gmail.com>` (إعداد محلي للـ repo) |
| الـ Design System | قواعد `aurora-dark-ui` كاملة، والـ gradient بألوان الغروب (عنبري ← وردي) بدل الأزرق ← الوردي |
| لغة الوثائق | `CLAUDE.md` و`Plan/` والـ ADRs بالعربي. README والمعمارية والأمان والـ pipeline spec بالإنجليزي، ومعاهم README عربي |

## المهام

### 1. تظبيط PR #1 (`m1/foundations`)
- شيل كل الكومنتات من كود Go، وشيل قاعدة `revive exported` اللي بتجبر doc comments.
- ضيف `funlen` (100 سطر) وحد طول الملف (500 سطر) لـ `.golangci.yml`.
- اكتب `CLAUDE.md` بالعربي، وفيه قواعد المشروع: التسمية، والـ envelope، وممنوع الكومنتات، وممنوع الإشارة لـ AI، والـ agents على sonnet، وaurora-dark-ui، و`Plan/`.
- انقل الـ ADRs من `docs/adr/` لـ `Plan/ADR/` وترجمها للعربي (agent بـ sonnet).
- عدّل `CONTRIBUTING.md`: الـ DCO sign-off بالاسم الحقيقي، وممنوع trailers بتاعة AI.
- **الملفات:** `internal/buildinfo/*.go`، `.golangci.yml`، `CLAUDE.md`، `CONTRIBUTING.md`، `docs/adr/*` ← `Plan/ADR/*`، `Plan/Progress/*`.

### 2. Design System «داو» على aurora-dark-ui
- خلفيات graphite مايلة للأزرق، وعمق بالإضاءة مش بالظلال، ومن غير زجاج blur على الكروت.
- gradient الغروب (عنبري ← وردي) يظهر بحد أقصى 3 مرات في الشاشة، والكلام عليه داكن.
- حدود edge-light للعناصر التفاعلية بس.
- لون واحد ثابت لكل نوع: نشر شغّال، وبيتبني، ومستني موافقة، وفشل، واترجّع.
- Poppins مع IBM Plex Sans Arabic، والوزن أقصاه 600.
- شكل العنصر حسب دوره (28 ← 16 ← 12 ← 10)، ومفيش hover lift.
- الـ components الخاصة بداو اتعملها port: الـ pipeline، والـ logs، والموافقات، ومحرر الأسرار. والـ icons الـ 32 والـ logo ثابتين.
- تحديث الـ artifact، ومراجعة Sabry قبل أي كود واجهة.

### 3. استكمال الـ server (`m1/server`)
- نكمّل من الشغل المحفوظ في الـ worktree، ونطبّق PascalCase في JSON وDB والـ envelope ومن غير كومنتات.
- agent بـ sonnet، والتحقق في الجلسة الرئيسية.

### 4. استكمال الواجهة (`m1/web`)
- بعد اعتماد الـ Design System الجديد بس.

### 5. Docker + compose (`m1/deploy`)

## مفتوح
- موافقة Sabry على الخطة دي.
- تفعيل Dependency graph وbranch protection من إعدادات الـ repo.
