# حالة داو: اللي اتعمل واللي لسه

**آخر تحديث:** 2026-10-28 · **main:** `0c5aeb0` (إصلاحات المراجعة الأمنية لـ M3.6)

الوثيقة دي نقطة البداية لأي جلسة جديدة. التفاصيل في [سجل التقدّم](Progress/README.md) والقرارات في [`ADR/`](ADR/README.md) والخطة الكاملة في [`docs/roadmap.md`](../docs/roadmap.md).

## الريبو والوصول

- الريبو `Khaymah/dhow`، واسمه القديم `Khaymah/dhow` بيعمل redirect.
- أدوات GitHub في الجلسات بتشتغل بالاسم القديم، ومابيوصلش منها أحداث للـ PRs، فالـ CI بيتراجع يدوي (`get_check_runs`).
- الـ proxy بيرفض مسح الـ branches ورفع الـ tags. الـ branches بتتمسح بعد الدمج بـ `delete-merged-branch.yml`، والـ tags بتتعمل من GitHub نفسه.

## اللي اتنفّذ واتعمله merge

| المرحلة | المحتوى | الـ PR |
|---|---|---|
| M0 | الـ design system والمعمارية ونموذج الأمان والـ ADRs | قبل الـ PRs |
| M1 | أساس الريبو والـ CI، وهيكل السيرفر، وواجهة الـ shell، والـ Docker والـ Compose، والـ binary الموقّع والمنصّب التفاعلي، وأدلة الـ reverse proxy، وتقسيم الصفحات | #1 #2 #3 #4 #11 #12 #14 |
| M2.1 | إعداد أول owner، وكلمة السر، والجلسات، والـ audit، وشاشات الهوية | #10 |
| M2.2 | TOTP وrecovery codes وpasskeys | #16 |
| M2.3 | المؤسسات والفرق والأدوار والدعوات | #18 |
| M2.4 | OIDC ودخول GitHub والـ API tokens والتفضيلات | #20 |
| M2.5 | التقوية والمراجعة الداخلية ([قايمة الفحص](../docs/security/auth-review-checklist.md)) | #23 |
| M3.1 | عقد الـ proto والـ codegen | #15 |
| M3.2 | تسجيل الـ runners وهويتهم والـ runner plane | #17 |
| M3.3 | الـ jobs والـ leases والـ fencing والـ reconciler | #19 |
| M3.4 | تخزين الـ logs والـ SSE | #21 |
| M3.5 | الـ runner binary والـ lease loop والـ masking (بـ driver وهمي) | #22 |
| M2.6 | سياسة الخطوة التانية للمؤسسة، والجلسة 7 أيام وخمول 4 ساعات للأدمن، وإيميلات تنبيه بيانات الدخول (ADR-0054) | #26 |
| M3.6 | driver الـ Podman rootless، والـ agent كـ PID 1، والـ seccomp، والحدود متحقق منها من الـ cgroup نفسه، والـ escape والـ limits suites في job `isolation` على `ubuntu-26.04` (ADR-0055) | #25 |
| تطوير محلي | `docs/development.md`، وسكربتات التجهيز `deploy/dev/setup.sh` و`setup.ps1`، وأهداف `make dev-*` | #27 |
| مراجعة M3.6 | إصلاحات المراجعة الأمنية المستقلة لـ M3.6 (ADR-0056): الخطوة بتخلص بكل الـ processes بتاعتها، وملفات الأسرار، ووضع `directory` مقفول، والـ image بالـ ID، والـ vsock مقفول، والـ uid map بيتفحص. وكمان السيرفر بيكمّل أسماء الـ images القصيرة، وتغيير الاسم لـ Khaymah بالكامل | #28 |
| تغيير الاسم لـ Khaymah | `193f80c` غيّر الاسم كنص جوه ملفات `.pb.go`، فكل binary كان بيعمل panic، والـ image بقى اسمها بحرف كبير. والإصلاح: `make gen` واسم الـ image `ghcr.io/khaymah/dhow` | #29 |

شرط خلاص M2 اتحقق: اختبارات الصلاحيات خضرا، ومفيش finding مفتوح Critical أو High. والمراجعة الخارجية اتنقلت لـ M9 (ADR-0040).

## شغّال ومش متعمله merge

| الشغل | الـ branch | الـ PR | اللي فاضل |
|---|---|---|---|
| M3.7 الـ egress proxy (ADR-0057، القسم 11 في سجل اليوم) | `feat/zealous-galileo-wnw3fx` | draft | مراجعة أمنية مستقلة، وjob الـ `isolation` يخضر |

M2.6 وM3.6 والتطوير المحلي وإصلاحات مراجعة M3.6 وتغيير الاسم اتدمجوا في 2026-10-28 (الأقسام من 6 لـ 10 في سجل اليوم).

**الخطوات قبل الـ merge لأي مرحلة:** الشغل، وبعدين `make check`، ومراجعة أمنية مستقلة، وإصلاح الـ findings، وPR، والـ CI أخضر، وsquash merge.

## اللي لسه في M3 (بالترتيب)

```
M3.6 ─► M3.7 ─┬─► M3.8
   │          ├─► M3.9
   │          └─► M3.11
   └─► M3.10
M3.12 (محتاجة M3.4 بس)        M3.13 في الآخر
```

| المرحلة | المحتوى |
|---|---|
| M3.7 | الـ egress proxy: parser للقايمة بـ fuzz، وCONNECT، والفلترة بعد الحل، وforwarder في الـ agent |
| M3.8 | gVisor كـ spike بشروط go/no-go، ولو فشل يتأجّل على الـ host بـ ADR |
| M3.9 | الإلغاء بالتدريج، وsuite الفوضى (`make runner-chaos`) |
| M3.10 | تغليف الـ runner: أرشيفات موقّعة، وsystemd، و`install` و`doctor` و`rotate-keys`، و`get.sh --component runner` |
| M3.11 | driver الـ Kubernetes والـ Helm chart، و`make helm-e2e` على kind |
| M3.12 | الواجهة: أجهزة التشغيل، وتوكن التسجيل، وقايمة الـ jobs بـ log حي |
| M3.13 | fuzz إضافي، ومراجعة تهديدات الـ runner plane، و`deploy/m3-demo/run.sh` لشرط الخلاص |

## بعد M3

M4 الـ pipelines ومزوّدي Git، وM5 الأسرار والبيئات، وM6 أهداف النشر، وM7 الواجهة الكاملة والإشعارات، وM8 الإصدارات المدفوعة، وM9 التقوية والمراجعة الخارجية، وM10 البيتا، وM11 الـ GA. التفاصيل وشروط الخلاص في [`docs/roadmap.md`](../docs/roadmap.md).

## بنود مفتوحة اتأجلت

- **الـ runner:** حقل سقف الموارد في `RegisterRequest`، ورفض `RESOURCE_REQUEST_EXCEEDS_RUNNER` وقت الإنشاء.
- **الحمل:** الـ token bucket لكل runner ورد «الـ pool مشغول» (ADR-0042). والإشعار بالـ labels لو الـ runners كتروا. وحدود الـ logs لكل instance مع HA.
- **التنضيف:** تنضيف `"RunnerSessions"` في M3.13. واحتفاظ الـ logs لكل مؤسسة في M4.
- **الهوية:** أول دخول بالـ IdP وقبول الدعوات بيه، وback-channel logout. وشاشة tokens المؤسسة في الواجهة. وحذف مؤسسة.
- **مع M4:** scopes المشروع والبيئة في الـ tokens والـ bindings.
- **الـ sandbox بعد ADR-0056:**
  - مفيش حد للـ I/O.
  - `socketcall` على 32-bit لسه مسموح.
  - وضع القرص `directory` مقفول لحد ما يبقى ليه quota حقيقية.
  - الخدمات اللي بتعيش طول الـ job (`services`) محتاجة ميزة لوحدها، لأن مفيش process بيعيش بعد خطوته.
- **`setup.ps1`** ماتجرّبش على Windows حقيقي (مفيش WSL2 على runners بتاعة GitHub).
- **الأمان:**
  - حد إيميلات التنبيه لكل مستخدم (فيه بس سقف 32 إرسال معلّق للسيرفر كله)، والتنبيهات تتنقل على queue الـ M7.
  - Trusted Types في الـ CSP مع M7 (F9).
  - المراجعة الخارجية في M9.
  - احتفاظ الـ audit 90 يوم.
  - GO-2026-5932 في `golang.org/x/crypto` لحد ما يطلع إصلاح.
- **التنصيب:** ربط `make proxy-check` بالـ CI، وتجربة certbot وCloudflare وIPv6 الحقيقيين.

## طريقة الشغل اللي ماشيين بيها

- كل مرحلة بيشتغل عليها agent في worktree تحت `.claude/worktrees/`، والتحقق في الحلقة الرئيسية بـ `make check`.
- بعدها مراجعة أمنية مستقلة، والـ Critical والـ High بيتصلّحوا في نفس الـ PR.
- الـ hooks مابتشتغلش جوه `.claude/worktrees`، فقواعدها بتتراجع يدوي قبل الـ commit.
- كل agent ليه Postgres على port لوحده وملف env في مجلد خاص بيه، عشان مايكتبوش فوق بعض.
- الـ Docker daemon ممكن يحتاج يتشغّل تاني بعد إعادة تشغيل الـ container، والديسك بيتملي فـ `go clean -cache` لما مفيش agent بيبني.
