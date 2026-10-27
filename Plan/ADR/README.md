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
| [0031](0031-web-structure-and-ui-components.md) | هيكل الواجهة بالـ Modules، وأسماء الـ hooks، وcomponents مكتوبة بإيدينا فوق Radix |
| [0032](0032-stacked-slides-and-themes.md) | الشرائح المتراكبة من اليمين، وثيمات النظام الأربعة، والثيم المخصص |
| [0033](0033-browser-sessions-invites-breached-passwords.md) | جلسات المتصفح بـ cookie في السيرفر، والدعوات برابط وSMTP اختياري، وفحص كلمات السر المسرّبة |
| [0034](0034-ui-direction-calm-and-organized.md) | اتجاه التصميم «هادي ومنظّم» |
| [0035](0035-identity-core-implementation.md) | تنفيذ نواة الهوية في M2.1: سلسلة الـ audit، والـ throttle، وإعلان الـ policy لكل route |
| [0036](0036-interactive-host-installer.md) | الـ installer التفاعلي على السيرفر (`dhow-server install`) |
| [0041](0041-runner-identity-and-registration.md) | هوية الـ runner وتسجيله: مفاتيح Ed25519 متثبّتة، وتوكن قصير، وإثبات امتلاك عند الحاجات الحساسة |
| [0042](0042-runner-protocol-and-leasing.md) | بروتوكول الـ runner: Connect بـ RPCs unary بس، وleases بـ fencing token، وتسليم at-least-once بإنهاء idempotent |
| [0043](0043-job-sandbox-rootless-podman.md) | الـ sandbox: Podman rootless افتراضيًا، وagent هو PID 1، وحدود cgroup v2 بتفشل مقفولة، وgVisor اختياري |
| [0044](0044-job-network-egress-proxy.md) | شبكة الـ job: `none` افتراضيًا، والـ egress من خلال proxy في الـ runner بقايمة hosts مسموحة |
| [0045](0045-secrets-transport-and-masking.md) | توصيل الأسرار للـ job: HPKE لمفتاح الـ runner، والذاكرة بس، والـ masking في الـ runner قبل أي حاجة تخرج |
| [0046](0046-job-logs-storage-and-streaming.md) | الـ logs: chunks متسلسلة من غير فراغات في جدول Postgres مقسّم بالأيام، وSSE بيكمّل من آخر seq |
| [0047](0047-job-state-machine-and-reconciliation.md) | حالات الـ job والـ attempt، والإلغاء بالتدريج، والـ reconciliation بقيود في قاعدة البيانات |
| [0048](0048-runner-packaging-and-kubernetes.md) | تغليف الـ runner: binary وsystemd ووضع `runner` في الـ installer، وHelm chart واحد، وpod لكل job على Kubernetes |

الأرقام من 0037 لـ 0040 محجوزة لقرارات M2 اللي جاية.
