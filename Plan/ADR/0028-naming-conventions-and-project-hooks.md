# ADR-0028: قواعد التسمية والأسلوب، وفرضها بـ Claude Code hooks

- **الحالة:** مقبول
- **التاريخ:** 2026-10-27

## السياق
قواعد Sabry موجودة في `sabry-claude-kit` وفي قواعد Deploy Center، وأول جزء من M1 اتكتب قبل ما تتطبّق. والمشروع public وهيبقى فيه مساهمين كتير بيستخدموا Claude Code، فالقواعد لو كانت مكتوبة بس هتتنسي.

## القرار
- أسماء الحقول في JSON بـ PascalCase، والـ response envelope هو `{Success, Data, Error}`.
- أسماء الجداول والأعمدة في PostgreSQL بـ PascalCase بين علامات تنصيص.
- Go بالأسلوب الرسمي، وTypeScript بـ `I`/`T`/`E` prefixes وarrow components وPascalCase للـ handlers والـ state.
- الكود من غير كومنتات إلا `IMPORTANT:` والـ directives. الملف 500 سطر بالكتير، والـ function 100 سطر.
- ممنوع أي إشارة لـ AI في الـ git، والـ commits بالإنجليزي وConventional.
- القواعد دي بتتفرض بـ hooks في `.claude/settings.json` (Python من غير اعتماديات)، ولكل قاعدة اختبار في `.claude/hooks/tests/` بيتشغّل في الـ CI.

## البدائل اللي اتدرست
- تكتفي بـ CLAUDE.md: بيتنسي، وده اللي حصل فعلاً.
- pre-commit hooks بتاعة git بس: بتمسك الخطأ متأخر بعد ما الكود اتكتب، ومش بتغطي قواعد التعديل نفسه.

## العواقب
- أي مساهم بـ Claude Code بياخد الرفض وقت التعديل ومعاه السبب والحل.
- PascalCase في Postgres محتاج علامات تنصيص في كل query، والـ hook بيمسك أي اسم مش متنصّص في الـ migrations.
- الـ hooks مش بديل عن lint والـ CI. هما الخط الأول، والـ CI هو الحَكَم.
