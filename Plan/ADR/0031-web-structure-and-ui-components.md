# ADR-0031: هيكل الواجهة بالـ Modules، وأسماء الـ hooks، وcomponents مكتوبة بإيدينا فوق Radix

- **الحالة:** مقبول
- **التاريخ:** 2026-10-27
- **بيعدّل:** [ADR-0023](0023-ui-stack.md) في نقطة shadcn/ui بس.

## السياق
السيرفر اتبنى بالـ Modular Monolith ([ADR-0030](0030-server-modular-monolith.md)). والـ Design System «داو» ([ADR-0029](0029-design-system-aurora-dusk.md)) فيه 37 component شكلهم بعيد عن shadcn/ui. وقواعد Sabry في `Stacks/NextJS.md` بتقول إن الـ hooks اسمها `UseX`.

## القرار
- **الهيكل:** الواجهة بنفس تقسيم السيرفر:
  - `Src/App`: الـ router والـ providers والـ layout.
  - `Src/Core/{Ui,Api,Hooks}`.
  - `Src/I18n/Locales/{Ar,En}.ts`.
  - `Src/Styles`.
  - `Src/Modules/<Module>/{Pages,Components,Hooks,Api,Store,Types}`.
  - module ميعملش import لـ module تاني، و`Core` ميعملش import لـ `Modules`.
- **التسمية:** PascalCase للفولدرات والملفات والـ components والـ state والـ handlers. الاستثناء الوحيد الـ hooks: `useX` و`useX.ts`.
- **الـ components:** في `Src/Core/Ui`، مكتوبة بإيدينا من الـ Design System. Radix للحاجات التفاعلية بس (Dialog وTabs وSwitch وCheckbox وTooltip)، ومن غير shadcn/ui.
- **الشغل القديم:** worktree `m1/web` القديم بيتمسح، بعد ما ناخد منه الإعدادات اللي تنفع.

## البدائل اللي اتدرست
- هيكل `Stacks/React.md` (pages وcomponents وhooks): كل فولدر هيكبر بكل الـ features مع بعض، وهيبقى شكل مختلف عن السيرفر.
- `UseX` زي NextJS.md: `eslint-plugin-react-hooks` بيتعرّف على الـ hooks من `use` الصغيرة، فقواعد الـ hooks هتتعطّل من غير ما حد ياخد باله. وده نوع الـ bug اللي كسر زرار الـ Rollback في Deploy Center.
- shadcn/ui: هنشيل أغلب الـ variants والألوان اللي جاية معاه، والـ components الخاصة بداو هتتكتب بإيدينا برضه.

## العواقب
- الـ hooks بتاعة المشروع بتفرض الهيكل واتجاه الـ imports والتسمية في `apps/web`، وبتمنع أي نص جوه JSX.
- إحنا اللي بنصون الـ 37 component، وكل واحد ليه اختبار للـ keyboard والـ focus والـ aria.
- أي استثناء تاني من PascalCase محتاج ADR جديد.
