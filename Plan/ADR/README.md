# سجلّ قرارات المعمارية (Architecture Decision Records)

| ADR | العنوان |
|---|---|
| [0001](0001-product-positioning.md) | تموضع المنتج: منصة CD ذاتية الاستضافة بتتكامل مع CI |
| [0002](0002-rewrite-with-v3-hotfix.md) | إعادة الكتابة من الصفر، مع إصلاح أمني عاجل لـ v3 بالتوازي |
| [0003](0003-go-for-server-and-runner.md) | Go للـ control plane والـ runner |
| [0004](0004-monorepo-layout.md) | Monorepo واحد |
| [0005](0005-api-rest-openapi-and-connect.md) | REST عام + OpenAPI؛ وConnect/gRPC للـ runners؛ وSSE للواجهة الحيّة |
| [0006](0006-postgres-default-redis-optional.md) | PostgreSQL افتراضيًا؛ وRedis اختياري |
| [0007](0007-job-isolation.md) | عزل الـ jobs: حاويات rootless افتراضيًا، وmicroVM اختياري |
| [0008](0008-pipeline-as-code-with-visual-editor.md) | Pipeline-as-code (`.dhow.yml`) مع محرّر مرئي |
| [0009](0009-container-image-plugins.md) | الـ Plugins عبارة عن خطوات (steps) على شكل container image |
| [0010](0010-build-or-ingest-artifacts.md) | البناء جوه Dhow أو استيراد الـ artifacts من CI خارجي |
| [0011](0011-deploy-targets.md) | أهداف النشر (deploy targets) في v1.0 |
| [0012](0012-git-providers.md) | مزوّدو Git عند الإطلاق |
| [0013](0013-authentication-methods.md) | طرق المصادقة |
| [0014](0014-secrets-management.md) | الأسرار: تشفير envelope مدمج؛ وVault/KMS ضمن Enterprise |
| [0015](0015-rbac-model.md) | RBAC: أدوار ثابتة + Teams؛ والأدوار المخصصة مدفوعة |
| [0016](0016-multi-org.md) | منظمات متعددة في التثبيت الواحد من اليوم الأول |
| [0017](0017-licensing-agpl-and-ee.md) | نواة AGPL-3.0 + مجلد `ee/` تجاري |
| [0018](0018-pricing-per-active-user.md) | الإصدارات المدفوعة بتتسعّر لكل مستخدم نشط |
| [0019](0019-no-community-limits.md) | مفيش حدود رقمية في Community |
| [0020](0020-managed-cloud-later.md) | control plane مُدار بعد استقرار الاستضافة الذاتية |
| [0021](0021-arabic-first-ui.md) | واجهة عربية أولًا |
| [0022](0022-navigation-model.md) | مساحة عمل واحدة + لوحات منزلقة (slide-over) + command palette |
| [0023](0023-ui-stack.md) | تقنيات الواجهة: React + Tailwind + shadcn/ui + Motion |
| [0024](0024-install-methods.md) | طرق التثبيت الرسمية |
| [0025](0025-notifications.md) | قنوات الإشعارات عند الإطلاق |
| [0026](0026-product-name.md) | اسم المنتج: Dhow |
| [0027](0027-public-repo-from-day-one.md) | مستودع عام من اليوم الأول |
| [0028](0028-naming-conventions-and-project-hooks.md) | قواعد التسمية والأسلوب، وفرضها بـ Claude Code hooks |
| [0029](0029-design-system-aurora-dusk.md) | الـ Design System على aurora-dark-ui بألوان الغروب، وغامق بس في الإصدار الأول |
| [0030](0030-server-modular-monolith.md) | هيكل السيرفر: Modular Monolith بتكييف Go |
