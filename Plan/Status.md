# حالة داو: اللي اتعمل واللي لسه

**آخر تحديث:** 2026-10-28 · **main:** `b4432f2` (M2.5)

الوثيقة دي نقطة البداية لأي جلسة جديدة. التفاصيل في [سجل التقدّم](Progress/README.md) والقرارات في [`ADR/`](ADR/README.md) والخطة الكاملة في [`docs/roadmap.md`](../docs/roadmap.md).

## الريبو والوصول

- الريبو `Khaymah/dhow`، واسمه القديم `Arab-OpenSource/dhow` بيعمل redirect.
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

شرط خلاص M2 اتحقق: اختبارات الصلاحيات خضرا، ومفيش finding مفتوح Critical أو High. والمراجعة الخارجية اتنقلت لـ M9 (ADR-0040).

## شغّال ومش متعمله merge

### M2.6: سياسة الخطوة التانية والجلسات والتنبيهات (`m2/session-policy`، draft PR #26)

الخطة في `Plan/M2.6-Plan.md` والقرارات في [ADR-0054](ADR/0054-second-factor-policy-sessions-and-credential-alerts.md) والقسم 6 في سجل اليوم.
- **T1:** إعداد `RequireSecondFactor` للمؤسسة، ومهلة 7 أيام، وبعدها `403 SECOND_FACTOR_REQUIRED` على الإجراءات الحساسة. التوسيع بيدّي مهلة للأدوار الجديدة بس.
- **T2:** الجلسة 7 أيام بالكتير (وكمان الجلسات القديمة)، والخمول 12 ساعة، أو 4 ساعات لأي حد owner أو admin، من الدور الحالي في كل طلب.
- **T3:** موديول `Alerts`: إيميل لـ 10 أحداث بيانات دخول لو الـ SMTP متظبط، في الخلفية ومن غير أسرار، والـ audit في كل الأحوال.

**الحالة:** الميزة خلصت. `make check` كامل أخضر، والواجهة خضرا، والمراجعة الأمنية المستقلة اتعملت (Medium واحد وكام Low، واتصلّحوا كلهم: F25 لحد F30 في قايمة الفحص)، وF10 وF11 وF12 اتقفلوا. الفرع فيه `origin/main`.
- لسه: مراجعة Sabry، وبعدها Ready for review عشان الـ CI يشتغل، وsquash merge.

### M3.6: driver الـ Podman والـ agent والـ seccomp (`m3/podman`، draft PR #25)

- اتعمل: طبقة `Agent`، وdriver الـ Podman rootless، والـ agent كـ PID 1، وقناة `Wire`، والـ seccomp، والـ escape والـ limits suites (`apps/runner/Test/Isolation` و`make runner-isolation`)، وjob الـ `isolation` في الـ CI، وADR-0055، والوثائق، والقسم 7 في سجل اليوم.
- لسه: الـ suite عمره ما اشتغل، لأن جهاز التطوير cgroup v1 ومفيهوش Podman. التحقق في job الـ `isolation` على GitHub Actions بـ `ubuntu-26.04` (قرار Sabry بعد ما `ubuntu-24.04` طلع فيه Podman 4.9.3). وبعدها المراجعة الأمنية المستقلة.
- مخاطر معروفة: كشف الـ OOM من `podman inspect`، وتفويض الـ cgroup تحت `systemd-run`. والاتنين هيبانوا من أول تشغيل.

**الخطوات قبل الـ merge لكل واحد:** تكملة اللي لسه، وبعدين `make check`، ومراجعة أمنية مستقلة، وإصلاح الـ findings، وPR، والـ CI أخضر، وsquash merge.

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
