# خطة M4: خطوات النشر ومزوّدي Git (Pipelines & Git providers)

**الحالة:** مقبولة. Sabry جاوب [الأسئلة المفتوحة](#أسئلة-مفتوحة-لـ-sabry) كلها بالتوصية يوم 2026-10-30. القرارات التقنية مفوَّضة ومكتوبة بالتوصية، وكل واحد منها بيبقى ADR في أول المرحلة بتاعته · **التاريخ:** 2026-10-30

## الهدف
شرط خلاص M4 في `docs/roadmap.md` بعد [ADR-0068](ADR/0068-first-production-release-scope.md):
- **مستودعات حقيقية على المزوّدين التلاتة** (GitHub App، وGitLab، وGit عام على Forgejo) بتشغّل pipelines فيها أكتر من job: DAG وmatrix، وartifacts بتعدّي من job للتاني، وcache بيتسترجع في التشغيل التاني.
- **webhook مزوّر أو متعاد بيترفض** على التلاتة، ومن غير ما يعمل run.
- **PR من fork بيمشي بالسياسة:** التعريف من الـ base، ومن غير أسرار ولا بيئات، والـ cache للقراية بس.
- **`dhow lint`** بيطلّع نفس الأخطاء اللي السيرفر بيطلّعها لنفس الملف.

المراجع:
- [ADR-0001](ADR/0001-product-positioning.md) و[ADR-0008](ADR/0008-pipeline-as-code-with-visual-editor.md) و[ADR-0009](ADR/0009-container-image-plugins.md) و[ADR-0010](ADR/0010-build-or-ingest-artifacts.md) و[ADR-0012](ADR/0012-git-providers.md) و[ADR-0030](ADR/0030-server-modular-monolith.md).
- M3 اللي بنبني عليه: [ADR-0043](ADR/0043-job-sandbox-rootless-podman.md) و[ADR-0045](ADR/0045-secrets-transport-and-masking.md) و[ADR-0047](ADR/0047-job-state-machine-and-reconciliation.md) و[ADR-0053](ADR/0053-runner-implementation.md) و[ADR-0057](ADR/0057-egress-proxy-implementation.md).
- `docs/pipeline-spec.md` و`docs/architecture.md` (الأقسام 3 و4 و8 و10 و11) و`docs/security-model.md`.

**قبلها:** M3.13 (fuzz ومراجعة تهديدات الـ runner plane والـ demo) بيخلص الأول، زي الترتيب في ADR-0068.

## النطاق
1. **`.dhow.yml`:** الـ parser بحدود، والـ validator، وCEL للشروط والقيم، والـ DAG والـ matrix، في package مشترك بين السيرفر والـ runner والـ CLI.
2. **`dhow lint`:** أول أمر في الـ CLI (`apps/cli`)، offline.
3. **المشاريع (Projects):** موديول جديد، وربط المستودع، وصلاحيات على مستوى المشروع (`"RoleBindings"` فيه `Scope = 'project'` من M2 ومستني الجدول).
4. **مزوّدي Git التلاتة:**
   - **GitHub App:** إنشاء الـ App من السيرفر بـ manifest، والـ installations، وتوكنات قصيرة، والـ webhooks، والـ check runs.
   - **GitLab** (GitLab.com والـ self-managed): توكن API على السيرفر بس، وwebhook بتوكن لكل مستودع، وdeploy key للـ checkout، وcommit statuses.
   - **Git عام:** deploy key بيتولّد في السيرفر ويظهر نصه العام بس، وhost key متثبّت، وwebhook URL بسر HMAC (بيغطي Gitea وForgejo).
5. **الـ runs:** الـ triggers (push وtag وpull_request وmanual وapi)، والـ DAG scheduler، والـ concurrency groups، وسياسة الـ forks، وتقارير الحالة للمزوّد.
6. **الـ runner:** أكتر من حاوية في نفس الـ attempt بـ workspace مشترك، وخطوة checkout مدمجة، وإضافات `uses:` (صور حاويات).
7. **الـ artifacts والـ cache:** blob store على القرص، ورفع وتنزيل من الـ runner، واحتفاظ وتنضيف.
8. **واجهة صغيرة:** إعداد المزوّدين، والمشاريع، وقايمة الـ runs وتفاصيلها، عشان أي مستخدم حقيقي يقدر يجرّب (الواجهة الكاملة M7).

## مش في M4
- **Bitbucket:** بعد أول نسخة (ADR-0068).
- **GitHub Enterprise Server:** بعد أول نسخة لو Sabry وافق على التوصية في [السؤال 5](#5-نسخ-المزوّدين).
- **الأسرار:** M5. الـ validator بيقبل `secrets.*`، بس أي job بيطلب سر بيفشل وقت الإدراج بـ `SECRETS_UNAVAILABLE` لحد M5. الاستثناء الوحيد هو credential الـ checkout اللي السيرفر بيولّده بنفسه.
- **البيئات وقواعد الحماية والموافقات:** M5. الـ validator بيقبل `environment:`، والـ job ده بيتعلّم `skipped` بـ `ENVIRONMENTS_UNAVAILABLE` لحد M5.
- **أهداف النشر والعلامة `mutating` الحقيقية:** M6. الـ trigger `artifact` (CI خارجي) في M6 مع الـ GitHub Action.
- **المحرر المرئي:** بعد أول نسخة. الـ JSON Schema بيتنشر للمحررات (VS Code) بس.
- **الـ trigger `schedule`:** بعد أول نسخة (قرار مفوَّض 13).
- **الـ polling لـ Git عام من غير webhooks:** حسب [السؤال 6](#6-git-عام-من-غير-webhook).
- **S3 والـ object storage البعيد:** حسب [السؤال 4](#4-تخزين-الـ-artifacts-والـ-cache).
- **الـ log segments في الـ blob store:** الـ logs بتفضل في Postgres في أول نسخة، ونرجع للقرار بعد اختبارات الحمل في M9.
- **الخدمات اللي بتعيش طول الـ job (`services:`):** ميزة لوحدها بعد أول نسخة.
- **سجل إضافات بناشرين موثّقين:** بعد GA (ADR-0009).

## المعمارية
```
 GitHub ─ webhook ─┐                        ┌──────────────────────── dhow-server ─────────────────────────┐
 GitLab ─ webhook ─┼──► proxy ──► /webhooks │ Modules/Sources     verify (HMAC | token), dedupe, normalize  │
 Forgejo ─ webhook ┘                        │      │ Core/Triggers (port, same tx)                          │
                                            │      ▼                                                       │
 browser ─────────────► proxy ──► /api/v1   │ Modules/Pipelines   fetch .dhow.yml, validate, runs, DAG     │
                                            │      │ Core/JobQueue (port, same tx)        ▲ Core/RunEvents │
                                            │      ▼                                      │ (port, same tx)│──► PostgreSQL 18
                                            │ Modules/Jobs        jobs, attempts, leases, logs (M3) ───────┤
 dhow-runner ─────────► proxy ──► /runner-api                                                              │
   Connect (unary)  Lease ─ Start ─ Heartbeat ─ BeginStep ─ Complete(outputs)   ArtifactService CacheService│
   raw HTTP         PUT/GET /runner-api/blobs/{sha256}  (lease-bound)  ──► Modules/Artifacts ──► DHOW_BLOB_DIR
                                            │ Modules/Sources ─► GitHub/GitLab API (netguard: no private IPs) │
                                            └──────────────────────────────────────────────────────────────┘

 runner host: one attempt = one workspace shared by several sandboxes, run one after the other
   [checkout]  image ghcr.io/khaymah/dhow-git@sha256 (pinned in the runner), agent runs git by argv,
               egress = the repo host only, credential = step-only secret file
   [run step]  the job image, /bin/sh -e -s, egress = jobs.<id>.network.egress
   [uses step] the plugin image@sha256, entrypoint by argv, inputs as DHOW_INPUT_*, egress = the job's
   every sandbox: --network=none --read-only --cap-drop=ALL, our seccomp, inspect-after-create (ADR-0043)
```

## إزاي الـ run بيتحوّل لـ jobs (من غير ما نعيد بناء M3)
الفكرة: **الـ pipeline طبقة فوق الـ jobs**، والـ job بتاع M3 بيفضل هو وحدة التنفيذ بكل ضماناته (الـ lease والـ fencing والإعادة قبل الـ mutation بس والإلغاء بالتدريج والـ reconciler).

1. **trigger:** `Sources` بيتأكد من الـ webhook ويحوّله لـ `Trigger` موحّد (المزوّد، والمستودع، والحدث، والـ ref، والـ sha، والـ PR، والـ actor كبيانات)، ويسلّمه لـ `Pipelines` من خلال الـ port `Core/Triggers` **جوه نفس الـ transaction** اللي سجّل فيه الـ delivery. يعني يا الاتنين يتسجّلوا يا ولا واحد.
2. **التعريف:** `Pipelines` بيعمل `"PipelineRuns"` بحالة `pending`، وبيجيب `.dhow.yml` من الـ sha الصح (الـ head، أو الـ base لو PR من fork)، وبيتحقق منه بـ `internal/pipelinespec`. لو فيه خطأ: الـ run بيبقى `failed` بـ `PIPELINE_INVALID` والأخطاء بمكانها (سطر وعمود)، والمزوّد بيشوف check فاشل.
3. **التوسيع:** كل job في الـ YAML، بعد الـ matrix، بيبقى صف في `"RunJobs"` بحالة `waiting`، ومعاه الـ `needs` بعد التوسيع.
4. **الإدراج:** أي `RunJob` كل الـ `needs` بتاعته `succeeded`، وشرط `if` بتاعه طلع `true`، بيتحوّل لـ `JobSpec` (نفس رسالة M3 بعد التوسيع اللي تحت) وبيتدرج بـ `Core/JobQueue.Enqueue` جوه نفس الـ transaction. الـ `"Jobs"` بياخد `"OriginKind" = 'pipeline'` و`"OriginId"` = الـ `RunJob`. لو الشرط `false`: `skipped`.
5. **التنفيذ:** M3 زي ما هو. الـ runner بياخد lease، وبينفّذ الـ checkout وبعده الخطوات، وبيرفع الـ artifacts، وبيبعت `Complete` ومعاه الـ outputs.
6. **الرجوع:** لما الـ job يوصل حالة نهائية، `Jobs` بينادي `Core/RunEvents.JobFinished` جوه نفس الـ transaction اللي نقل الحالة. `Pipelines` بيحدّث الـ `RunJob`، وبيقيّم اللي بعده (خطوة 4)، وبيعلّم اللي اتقطع عنه الطريق `skipped`، وبيقفل الـ run لما كله يخلص. وبيكتب الحالة المطلوبة للمزوّد في جدول `Sources` بالـ port `Core/CommitStatus`، والإرسال نفسه بره الـ transaction بـ worker بيعيد.
7. **الإلغاء:** `POST .../runs/{runId}/cancel` بيلغي الـ `RunJobs` اللي لسه `waiting`، وبينادي `Core/JobQueue.Cancel` على الـ jobs الشغّالة (نفس الإلغاء بالتدريج بتاع ADR-0047).
8. **الإعادة:**
   - إعادة البنية التحتية (attempt ضاع قبل أي mutation) جوه الـ job نفسه زي M3، والـ run مابيحسّش بيها.
   - «أعد تشغيل الفاشل» من المستخدم بيعمل `"Jobs"` جديد لنفس الـ `RunJob` (ADR-0047: حالات الـ job النهائية ما بتتغيّرش). والتاريخ كله في `"Jobs"` اللي `"OriginId"` بتاعها هو الـ `RunJob`.
9. **شبكة الأمان:** reconciler في `Pipelines` كل 5 ثواني بـ `SKIP LOCKED`، بيمسك أي `RunJob` حالته مش متطابقة مع الـ job بتاعه (لو حصل bug)، وأي run `pending` عدّى دقيقتين من غير تعريف.

### التوسيع من YAML لـ `JobSpec`
| في `JobSpec` | من فين |
|---|---|
| `image` | `jobs.<id>.image`، والاسم القصير بيتكمّل زي M3 |
| `checkout` (جديد) | المستودع والـ sha والـ ref من الـ run، وشكل الـ credential من المزوّد. الـ credential نفسه بيتولّد وقت `Start` مش وقت الإدراج |
| `steps` | كل خطوة `run` زي M3، وكل `uses` بقى `Step.plugin` (جديد)، و`mutating` دايمًا `false` لحد M6 |
| `env` | `env` بتاع الـ job والخطوة بعد تقييم CEL، وزيادة عليهم `DHOW_*` (الـ sha والـ ref والـ run والمشروع) كبيانات |
| `secrets` | فاضي في M4 إلا credential الـ checkout (اسم محجوز `dhow.git.checkout`) |
| `network.egress` | `jobs.<id>.network.egress`. host المستودع مش بيتضاف هنا، بيتضاف لحاوية الـ checkout بس |
| `resources` | `jobs.<id>.resources` (مفتاح جديد في الـ spec)، أو الافتراضي من السيرفر |
| `labels` و`isolation` و`timeout_seconds` | `defaults.runner` و`jobs.<id>.runner` و`timeout`. `isolation` في أول نسخة `container` بس |
| `artifacts` و`inputs` (جداد) | `jobs.<id>.artifacts` للرفع، وartifacts الـ `needs` بالـ digest للتنزيل |
| `cache` (جديد) | تعبير الـ key (بيتقيّم في الـ runner عشان `hash_files`) والـ paths والـ scope ووضع القراية بس |

## القرارات المفوَّضة (كل واحد بيبقى ADR في أول مرحلته)
| # | القرار | ليه في سطر | المرحلة |
|---|---|---|---|
| 1 | موديولات جديدة: `Projects` و`Sources` و`Pipelines` و`Artifacts`. وبينهم ports في `Core` (`Triggers` و`JobQueue` و`RunEvents` و`CommitStatus` و`ArtifactIndex`) بتتنفّذ في الموديول وبتتركّب في `App`، وبتشتغل جوه نفس الـ transaction | الـ EventBus مافيهوش outbox، وحدث بيضيع بين الـ commit والنشر معناه run واقف. ونفس نمط `Core/RunnerAuth` الموجود | M4.3 |
| 2 | السيرفر بيقرا `.dhow.yml` من API المزوّد لـ GitHub وGitLab، ولـ Git عام بـ «discovery job» على runner بيعمل checkout ويرجّع الملف كـ output | مفيش git في صورة السيرفر (distroless)، ومفيش parsing لـ packfiles غريبة في الـ control plane | M4.8 |
| 3 | الـ checkout خطوة مدمجة في حاوية لوحدها من صورة بتاعتنا متثبّتة بالـ digest جوه الـ runner. الـ agent بيشغّل `git` بـ argv و`--`، والـ credential ملف للخطوة دي بس، وعمره ما بيبقى في الـ URL ولا الـ argv | الـ credential مابيوصلش لحاويات خطوات المستخدم، وصورة المستخدم مش محتاجة git | M4.7 |
| 4 | الـ attempt بقى أكتر من حاوية بالتتابع على workspace مشترك (tmpfs volume) بنفس الـ uid map. بيبدأ بـ spike بشروط زي M3.8 | الـ plugins والـ checkout محتاجين صور مختلفة، والملفات لازم تبقى بنفس المالك | M4.7 |
| 5 | الـ plugin بيعلن الـ manifest في label على الصورة (`io.dhow.plugin.manifest`). الـ runner بيقراه بعد السحب وبيتحقق من `with:` قبل التشغيل، والسيرفر بيتحقق من الشكل بس | السيرفر مابيسحبش صور. والتحقق الكامل وقت الـ lint بييجي مع السجل بعد GA | M4.7 |
| 6 | الـ artifacts والـ cache بيترفعوا وبيتنزّلوا بـ HTTP عادي على `/runner-api/blobs/{sha256}` مربوط بالـ lease، مش Connect | Connect عندنا unary بحد 1 MiB، والـ artifact ممكن يبقى جيجات. والـ digest بيتأكد وإحنا بنكتب | M4.9 |
| 7 | `${{ }}` ممنوع جوه `run:`، والـ validator بيقول استخدم `env:` | ده حقن الأوامر المشهور في GitHub Actions. القيمة توصل كمتغيّر بيئة، عمرها ما تبقى جزء من الـ script | M4.1 |
| 8 | الـ anchors والـ aliases والـ merge keys والـ tags المخصّصة ممنوعين في `.dhow.yml` | billion laughs ومعاني مخفية. والملف اللي بيتراجع في PR لازم يتقري زي ما هو | M4.1 |
| 9 | الـ validator بـ Go هو المصدر، و`packages/spec/dhow.schema.json` مكتوب بإيد ومعاه اختبار بيتأكد إن كل حقل في الـ Go موجود في الـ schema والعكس | من غير مكتبة JSON Schema في الـ runtime، والـ schema للمحررات بس | M4.1 |
| 10 | الـ PR بيتشغّل على sha الـ head، مش على merge ref | الـ sha هو اللي بيظهر في الـ check، وثابت لو الـ base اتحرك | M4.8 |
| 11 | الـ webhook تلميح، والحقيقة من المزوّد: لـ GitHub وGitLab بنتأكد من الـ API إن الـ sha هو رأس الـ ref (ولو اتحرك الـ run بيبقى `superseded`)، ولـ Git عام الـ checkout بيتأكد إن الـ sha موجود في تاريخ الـ ref | سر webhook متسرّب مايقدرش يشغّل commit قديم على `main` (ده deploy لنسخة قديمة في M6) | M4.4 لـ M4.6 |
| 12 | credentials المزوّدين (مفتاح الـ App وتوكن GitLab والـ deploy keys وأسرار الـ webhooks) متشفّرة بـ `Core/Crypto.Sealer` بمفتاح مشتق من `DHOW_SECRET_KEY` بـ label لكل نوع، لحد ما M5 ينقلهم للتشفير المغلّف | مفيش تخزين صريح لحد M5. وتغيير `DHOW_SECRET_KEY` بيكسرهم زي تسجيل الـ runners، ومكتوب | M4.4 |
| 13 | `schedule` بعد أول نسخة، و`artifact` في M6 | تقليل النطاق. والـ validator بيرفضهم برسالة واضحة بدل ما يتجاهلهم | M4.1 |
| 14 | تقارير الحالة: check runs على GitHub (من غير صلاحية statuses)، وcommit statuses على GitLab، ومفيش على Git عام (إشعارات M7 بتغطيه) | أقل صلاحيات ممكنة | M4.5 وM4.6 |
| 15 | الـ webhooks على `/webhooks/...` بره `/api/v1`، بـ middleware خاصة ونوع policy جديد `policy.Webhook` | زي `/runner-api/`: من غير جلسة ولا CSRF، والسر هو المصادقة | M4.4 |
| 16 | من غير `go-github` ولا `ghinstallation` ولا مكتبة JWT ولا `go-git`: الـ API calls القليلة وتوقيع RS256 من الـ standard library | سطح أقل، ونفس قاعدة M3 | M4.5 |

## مخطط `.dhow.yml` والـ validator
الكود في `internal/pipelinespec` (مشترك بين السيرفر والـ runner والـ CLI)، والـ schema للمحررات في `packages/spec/dhow.schema.json`، والمرجع للمستخدم في `docs/pipeline-spec.md` (بالنسختين).

### الـ parsing (YAML مش موثوق)
- **الحجم:** الملف 256 KiB بالكتير، وأي scalar 64 KiB، والعمق 32، وعدد الـ nodes 50,000.
- **مرحلتين:** `go.yaml.in/yaml/v3` لـ `yaml.Node` الأول، والفحص على الشجرة: مفيش alias ولا anchor ولا `<<` ولا tag غير الأساسية، ومفيش مفتاح مكرر. وبعدها decode لـ structs بـ `KnownFields(true)`، فأي مفتاح مش معروف خطأ.
- **الأخطاء:** فيها المسار (`jobs.build.steps[2].run`) والسطر والعمود، ومن غير ما تكرر القيمة نفسها (ممكن حد لزق سر بالغلط).
- **الأنواع:** المدد (`20m`) والأحجام (`2Gi`) بـ parser بتاعنا بحدود، مش `time.ParseDuration` على أي حاجة.

### الـ validator
- **`version: 1`** بس.
- **`on`:** `push` و`tag` و`pull_request` و`manual` (inputs بأنواع: string وboolean وchoice، 20 بالكتير) و`api`. و`schedule` و`artifact` مرفوضين بـ `TRIGGER_NOT_SUPPORTED_YET`.
- **الفلاتر:** `branches` و`tags` و`paths` بـ glob بتاعنا (`*` و`**` و`?` بس، من غير regex)، 50 pattern بالكتير.
- **الحدود:** 64 job بالكتير قبل الـ matrix، و256 `RunJob` بعده، و100 خطوة في الـ job (زي `"AttemptSteps"`)، وحجم الـ `JobSpec` بعد التوسيع تحت `jobspec.MaxSpecBytes`.
- **الأسماء:** id الـ job بـ `^[a-z][a-z0-9_-]{0,62}$`، ومفيش اتنين بنفس الاسم.
- **الصور:** `uses:` لازم بالـ digest دايمًا. و`image:` بالـ digest أو بالـ tag لو الـ runner مسموح له (`DHOW_RUNNER_ALLOW_UNPINNED_IMAGES`)، وقاعدة البيئات تفرض الـ digest في M5.
- **الـ egress:** نفس `jobspec.ParseEgress` بتاع M3.
- **السياسة:** `secrets.*` جوه `if` خطأ، و`${{ }}` جوه `run:` خطأ (قرار 7)، و`environment:` بيتقبل ويتأجّل لـ M5.
- **مستويات الخطأ:** `error` بيمنع التشغيل، و`warning` (زي صورة من غير digest) بيظهر في الـ lint والواجهة.

### CEL
- **المكتبة:** `github.com/google/cel-go` v0.30.0، وenv فيه المتغيرات المعلنة بس (`git` و`project` و`vars` و`inputs` و`matrix` و`needs` و`env`)، وأي متغير تاني خطأ وقت الـ compile.
- **الحدود:** طول التعبير 2 KiB، و`cel.CostLimit`، وعمق الـ parser، وcontext بمهلة 10 ms لكل تقييم. ومفيش أي دالة بتعمل I/O على السيرفر.
- **الأنواع:** `if` لازم `bool`، و`concurrency.group` و`cache.key` وقيم `env` و`with` لازم `string`. والفحص وقت الـ validation مش وقت التشغيل.
- **الدوال:** `startsWith` و`contains` و`matches` (RE2) من CEL نفسه، و`hash_files()` بتاعتنا وبتتقيّم في الـ runner بس (بعد الـ checkout، جوه الـ workspace، ومن غير ما تخرج منه).
- **القيم من Git بيانات:** رسالة الـ commit واسم الفرع واسم الكاتب متغيرات `string`، والنتيجة عمرها ما بتتعمل parse تاني كتعبير أو كـ YAML أو كـ shell.
- **مفيش مفسّر عام:** CEL بس، زي القاعدة في `CLAUDE.md`. ومفيش `text/template` ولا JavaScript ولا shell expansion في السيرفر.

### الـ DAG والـ matrix
- **`needs`:** الدورات بتترفض وقت الـ validation (Kahn)، والـ job اللي بيشاور على job مش موجود خطأ.
- **الـ matrix:** حاصل ضرب كارتيزي، و`include` و`exclude`، و`max_parallel`. والمفاتيح والقيم strings أو أرقام أو booleans بس. وأي `RunJob` اسمه `build (node=20, os=debian)`، والـ key ثابت ومرتّب.
- **`needs` على job فيه matrix:** معناها كل النسخ، والـ outputs بتاعته بتتجمّع في map بالـ key.
- **الفشل:** أي `need` فشل أو اتلغى، كل اللي بعده `skipped` بـ `NEEDS_FAILED`.

### الإضافات (صور الحاويات)
- **المرجع:** `uses: registry/name@sha256:...`، والـ inputs في `with:` (32 بالكتير، والقيمة 32 KiB).
- **الـ manifest:** label على الصورة فيه الـ inputs بأنواعها (مطلوب واختياري والافتراضي) والـ outputs والـ entrypoint. الـ runner بيقراه بـ `podman image inspect` بعد السحب بالـ digest، وبيرفض الخطوة بـ `PLUGIN_INPUT_INVALID` لو `with:` مش مطابق، قبل أي تشغيل.
- **التشغيل:** حاوية لوحدها بنفس التقوية وعلى نفس الـ workspace. الـ inputs كـ `DHOW_INPUT_<NAME>`، والـ outputs في ملف `$DHOW_OUTPUT` (`key=value`، 64 مفتاح و1 KiB لكل قيمة)، والـ entrypoint بـ argv من غير shell.
- **الـ egress:** بتاع الـ job. الـ plugin مابيقدرش يوسّعه.
- **إضافات بتاعتنا في M4:** مفيش غير الـ checkout المدمج. `deploy-ssh` و`deploy-compose` في M6.

### الـ artifacts والـ cache
- **الـ artifact:** tar.gz حتمي (ترتيب ثابت، وأوقات صفر، ومن غير uid/gid، وsymlinks جوه الـ workspace بس، ومن غير device files)، وبيتسمّى بالـ sha256 بتاعه. الحجم بحد (`DHOW_ARTIFACT_MAX_BYTES`)، والاحتفاظ افتراضيًا 30 يوم.
- **في الـ job اللي بعده:** الـ artifacts بتاعة الـ `needs` بتتنزّل وبتتفك جوه `/workspace` قبل أول خطوة، والـ digest بيتأكد قبل الفك، والفك بيرفض `..` والمسارات المطلقة والروابط لبره.
- **الـ cache:** الـ key بيتقيّم في الـ runner. الـ scope هو المشروع والفرع، والقراية بترجع للفرع الافتراضي لو مفيش. والحفظ بعد نجاح الـ job بس ولو الـ key ماكانش موجود. والحد لكل مشروع بيمسح الأقدم استخدامًا.
- **مين يكتب فين:** push على فرع بيكتب في scope الفرع ده بس. وPR من نفس المستودع بيكتب في scope الـ PR. وPR من fork مابيكتبش خالص. فمفيش طريقة PR يسمّم cache الـ `main`.

### `dhow lint`
- **`apps/cli/cmd/dhow`**، والهيكل `apps/cli/internal/{App,Lint,Output}`، والـ hook بيتوسّع للمسار ده.
- **`dhow lint [path]`:** افتراضيًا `.dhow.yml` في المجلد الحالي، و`--format text|json`، وكود الخروج 0 أو 1 (أخطاء) أو 2 (الملف مش موجود أو مش متقري).
- **نفس الـ validator** بتاع السيرفر بالظبط، offline، من غير اتصال ومن غير التحقق من manifests الـ plugins.
- **اللغة:** الرسايل بالعربي المبسّط أو الإنجليزي حسب `LANG` أو `--lang` (ADR-0062)، والـ JSON بأكواد ثابتة (`UPPER_SNAKE`) عشان المحررات.
- **التوزيع:** أرشيفات `dhow` في `release.yml` لـ linux وdarwin وwindows على amd64 وarm64، مع `SHA256SUMS` وcosign، و`get.sh --component cli`.

### سياسة PRs من الـ forks
- **الكشف:** GitHub: `head.repo.id != base.repo.id` أو الـ head repo اتمسح. GitLab: `source_project_id != target_project_id`. وGit عام مافيهوش PRs.
- **التعريف:** من sha الـ base، فالـ PR مايقدرش يغيّر الـ triggers ولا الـ egress ولا الصور اللي بتشتغل.
- **الكود:** sha الـ head بيتعمله checkout كبيانات، ومابيتقريش منه أي إعداد.
- **ممنوع:** الأسرار (حتى بعد M5)، والبيئات (`skipped` بـ `FORK_PR_NO_ENVIRONMENT`)، وكتابة الـ cache، واستخدام الـ artifacts في أي deploy (بتتعلّم `Untrusted`).
- **الموافقة والـ runners:** حسب [السؤال 1](#1-سياسة-prs-من-الـ-forks).
- **الإعادة من المزوّد** (زرار re-run في GitHub): مسموحة لغير الـ forks، ولـ PR من fork محتاجة نفس الموافقة.

## مزوّدي Git

### الطبقة المشتركة
- **`internal/gitref`:** التحقق من الـ refs بقواعد `git check-ref-format` (مفيش `..` ولا حروف تحكم ولا `-` في الأول ولا `@{` ولا `.lock` في الآخر، و255 byte بالكتير)، والأشكال المسموحة بس: `refs/heads/*` و`refs/tags/*` و`refs/pull/<n>/head` و`refs/merge-requests/<n>/head`. والـ sha 40 أو 64 حرف hex صغير.
- **`internal/gitsource`:** التحقق من عنوان المستودع:
  - `https://` و`ssh://` بس، والشكل `git@host:path` بيتحوّل لـ `ssh://`.
  - مفيش `file://` ولا `ext::` ولا `fd::`، ومفيش password في الـ URL، ومفيش `-` في أول الـ host أو المسار، ومفيش `%` لحروف تحكم ولا مسافات ولا سطر جديد.
  - الـ host بنفس قواعد `jobspec.ParseEgress` (ASCII صغير وpunycode، ومفيش IP literal).
- **`internal/netguard`:** فحص العناوين اللي بنتصل بيها من السيرفر، متشال من `apps/runner/internal/Egress/Egress.Ranges.go` عشان السيرفر والـ runner يستخدموا نفس القايمة:
  - بنرفض loopback والشبكات الخاصة وlink-local وCGNAT والـ metadata وmulticast، وIPv4 جوه IPv6.
  - الاتصال بالعنوان اللي اتفحص نفسه (`net.Dialer.Control`)، فالـ DNS rebinding مايعدّيش.
  - من غير redirects، وTLS إجباري (`http` بـ `DHOW_DEV` بس)، ومهلة 10 ثواني، والرد 5 MiB بالكتير.
  - `DHOW_GIT_ALLOW_PRIVATE_CIDRS` بيفتح نطاقات معيّنة لـ GitLab أو Gitea على شبكة داخلية، وبيتسجّل في الـ audit لو اتغيّر من اللوحة.
- **interface `Provider`** في `Modules/Sources`: `VerifyWebhook` و`ParseEvent` و`FetchFile` و`ResolveRef` و`MintCheckoutCredential` و`ReportStatus`. كل مزوّد بيطبّق اللي يقدر عليه، والباقي بيرجّع `ErrUnsupported` معروف.
- **الـ webhooks:**
  - **الترتيب:** الحد على الحجم (`DHOW_WEBHOOK_MAX_BYTES`، افتراضيًا 5 MiB)، وبعده التحقق من التوقيع على الـ body الخام بـ `hmac.Equal`، وبعدها بس الـ JSON.
  - **الـ dedupe:** delivery id فريد في `"WebhookDeliveries"`. التكرار بيرجّع `200` من غير أي أثر، فإعادة الإرسال من المزوّد آمنة.
  - **الرد:** `202` لأي webhook صح حتى لو اتجاهل (مستودع مش مربوط أو حدث مش مهم)، و`401` بنفس الرد لأي توقيع غلط أو مستودع مش موجود، عشان مانكشفش إيه اللي موجود.
  - **حد الطلبات:** لكل IP ولكل hook id بـ `Core/RateLimit`.
  - **الـ payload:** مابيتخزّنش. اللي بيتخزّن الـ metadata والـ `Trigger` الموحّد بس، لأن الـ payload فيه إيميلات وأسماء.

### GitHub App
- **إنشاء الـ App** (مرة واحدة للـ instance، من «إعدادات السيرفر» بصلاحية `instance.settings.manage`):
  - السيرفر بيبني manifest: الاسم، والـ webhook URL `<DHOW_PUBLIC_URL>/webhooks/github`، والـ callback، وsetup URL.
  - **الصلاحيات:** `contents: read` و`metadata: read` و`checks: write` و`pull_requests: read`.
  - **الأحداث:** `push` و`pull_request` و`check_run` و`installation` و`installation_repositories`.
  - **«Request user authorization during installation»** شغّال (عشان ربط الـ installation تحت).
  - الأدمن بيتحوّل لـ GitHub بـ `state` مربوط بجلسته (نفس تصميم الـ OIDC flow: cookie، والـ hash بس في قاعدة البيانات، و10 دقايق).
  - GitHub بيرجّع `code`، والسيرفر بيبدّله بـ `POST /app-manifests/{code}/conversions`، وبيخزّن الـ app id والـ client id والمفتاح الخاص وسر الـ webhook وclient secret متشفّرين (قرار 12). ومفيش أي منهم بيرجع في أي response.
- **ربط installation بمؤسسة:**
  - أدمن المؤسسة بيدوس «اربط GitHub»، فبيتحوّل لصفحة تنصيب الـ App بـ `state` موقّع (المؤسسة والمستخدم وnonce).
  - GitHub بيرجّع `installation_id` و`code` الخاص بالمستخدم.
  - **الحماية من سرقة installation:** الـ ids متسلسلة، فمؤسسة على نفس الـ instance ممكن تحاول تربط installation مؤسسة تانية وتقرا مستودعاتها. عشان كده السيرفر بيبدّل الـ `code` بتوكن مستخدم، ويتأكد إن `GET /user/installations` فيه الـ id ده، وبعدين يرمي التوكن. والـ installation مربوط بمؤسسة واحدة بس (unique).
  - `installation.deleted` و`suspend` بيقفلوا الربط ويعلّموا المشاريع.
- **التوكنات:**
  - JWT بـ RS256 (عمره 9 دقايق، و`iat` قبلها بـ 60 ثانية) من الـ standard library.
  - توكن الـ API للـ installation في الذاكرة لحد 50 دقيقة.
  - توكن الـ checkout بيتولّد وقت `Start` لكل attempt، محصور في المستودع ده و`contents: read`، ومابيتخزّنش في قاعدة البيانات خالص، وبيروح للـ runner مشفّر بـ HPKE زي أي سر (ADR-0045) وبيتضاف للـ masker.
- **قراية التعريف:** `GET /repos/{o}/{r}/contents/{path}?ref={sha}` بـ `Accept: application/vnd.github.raw`، وبالحد على الحجم.
- **الـ webhooks:** `X-Hub-Signature-256` بسر الـ App، و`X-GitHub-Delivery` للـ dedupe، والـ installation لازم مربوط والمستودع مربوط بمشروع.
- **الـ check runs:** check واحد لكل run باسم الـ pipeline، و`details_url` للـ run في داو. والملخص فيه أسماء الـ jobs وحالاتها بس، عمره ما بيبقى فيه logs. و`check_run.rerequested` بيعيد الفاشل (بسياسة الـ forks).

### GitLab
- **الاتصال** (لكل مؤسسة، بصلاحية `git.connect`):
  - `BaseUrl` (افتراضيًا `https://gitlab.com`، أو self-managed بقواعد `netguard`) وتوكن API بـ scopes `api` و`read_repository`، للكتابة بس ومتشفّر.
  - عند الحفظ: `GET /api/v4/personal_access_tokens/self` بيقرا النوع والـ scopes وتاريخ الانتهاء. والـ scopes الناقصة بترفض الحفظ، والانتهاء القريب بيبان في الواجهة وفي الـ audit.
  - أنواع التوكن المقبولة حسب [السؤال 8](#8-توكن-gitlab).
  - **التوكن ده على السيرفر بس، عمره ما بيوصل لأي runner.**
- **ربط المشروع:** اختيار المشروع، وبعدها السيرفر بيعمل بالـ API:
  - webhook (`POST /projects/:id/hooks`) بتوكن عشوائي 32 byte لكل مستودع، وpush وtag وmerge requests، و`enable_ssl_verification`.
  - deploy key للقراية بس (`POST /projects/:id/deploy_keys`، `can_push: false`) بيتولّد في السيرفر.
  - لو التوكن مايسمحش، الواجهة بتعرض الخطوات اليدوية (الـ URL والنص العام للمفتاح، والسر مرة واحدة).
- **الـ webhooks:**
  - `X-Gitlab-Token` بيتقارن بـ `hmac.Equal`، و`X-Gitlab-Event-UUID` للـ dedupe (و`Idempotency-Key` لو موجود).
  - التوكن ده مش توقيع على الـ body، فالتأكد من الـ sha والـ ref بالـ API (قرار 11) إجباري هنا قبل أي run.
- **قراية التعريف:** `GET /projects/:id/repository/files/:path/raw?ref={sha}`.
- **الـ checkout:** بـ SSH بالـ deploy key وhost key متثبّت (زي Git عام)، ففيه مسار checkout واحد للاتنين.
- **الحالة:** `POST /projects/:id/statuses/:sha` بالحالات `pending` و`running` و`success` و`failed` و`canceled`، والـ `target_url` للـ run.

### Git عام (Gitea وForgejo وأي Git)
- **الربط:** عنوان SSH للمستودع (`ssh://git@host:port/path.git` أو `git@host:path`)، أو `https://` لمستودع عام من غير credentials.
- **الـ deploy key:**
  - Ed25519 بيتولّد في السيرفر (`crypto/ed25519` و`golang.org/x/crypto/ssh` للصيغة).
  - النص الخاص متشفّر ومابيرجعش أبدًا، والنص العام بيظهر في الواجهة والـ API عشان المستخدم يضيفه للقراية بس.
  - «جدّد المفتاح» بيعمل مفتاح جديد ويظهر نصه العام، والقديم بيفضل شغّال لحد ما المستخدم يأكّد إنه ضاف الجديد، أو 24 ساعة.
- **الـ host key:**
  - السيرفر بيعمل SSH handshake بس (بقواعد `netguard`، ومن غير مصادقة) ويعرض بصمة `SHA256:` للأدمن يأكّدها.
  - بعد التأكيد بتتثبّت. ولو اتغيّرت، الـ checkout بيفشل `HOST_KEY_MISMATCH` لحد ما أدمن يأكّد البصمة الجديدة.
- **الـ webhook:**
  - URL `<DHOW_PUBLIC_URL>/webhooks/git/{HookId}`، والـ `HookId` عشوائي 128 bit مش الـ id بتاع المستودع، والسر 32 byte بيظهر مرة واحدة (زي توكن تسجيل الـ runner) وبيتجدّد بزرار.
  - **التوقيعات المقبولة:** `X-Gitea-Signature` و`X-Forgejo-Signature` و`X-Hub-Signature-256` (HMAC-SHA256 على الـ body)، والصيغة بتاعتنا لأي سكربت: `X-Dhow-Signature: t=<unix>,v1=<hex>` على `t + "." + body`، ومعاها مهلة ±5 دقايق.
  - **الـ delivery id:** `X-Gitea-Delivery` أو `X-Forgejo-Delivery` أو `X-Dhow-Delivery`، وwebhook من غيره بيترفض.
  - **الـ payload:** push وtag بصيغة Gitea/Forgejo، أو الصيغة بتاعتنا `{"Ref": "...", "Sha": "..."}`.
- **التعريف:** بالـ discovery job (قرار 2)، والـ checkout بيتأكد إن الـ sha موجود في تاريخ الـ ref (قرار 11).

### الـ checkout نفسه (في الـ runner)
- **الأمر:** `git init`، وبعدين `git fetch --depth=<n> --no-tags -- <url> <ref>`، والـ agent بيتأكد إن `FETCH_HEAD` هو الـ sha أو إن الـ sha في تاريخه (`git merge-base --is-ancestor`). لو الـ ref اتحرك والـ sha مش موجود، بيجرّب fetch بالـ sha، ولو فشل: `CHECKOUT_REF_MOVED`.
- **الإعدادات:**
  - `-c protocol.allow=never -c protocol.https.allow=always -c protocol.ssh.allow=always` و`-c core.hooksPath=/dev/null` و`-c submodule.recurse=false` و`-c http.followRedirects=false` و`-c transfer.fsckObjects=true`.
  - ومعاها `GIT_CONFIG_NOSYSTEM=1` و`GIT_TERMINAL_PROMPT=0` و`GIT_LFS_SKIP_SMUDGE=1` و`HOME` فاضي.
- **HTTPS:** `GIT_ASKPASS` لـ helper بيقرا التوكن من ملف السر بتاع الخطوة. والـ username `x-access-token` لـ GitHub.
- **SSH:** `GIT_SSH` لـ helper (مش `core.sshCommand` لأنه بيتعمله parse بالـ shell) بيشغّل `ssh` بـ argv ثابت: `StrictHostKeyChecking=yes` وملف `known_hosts` فيه السطر المتثبّت بس، و`IdentitiesOnly=yes`، والمفتاح من ملف السر، و`ProxyCommand` للـ helper `connect` بتاع M3 (الـ host اتحقق منه قبل كده).
- **الـ submodules والـ LFS:** مقفولين في أول نسخة. والمستخدم يقدر يعملهم في خطوة بنفسه، بالـ egress اللي هو حدده.

## الأمان
| الخطر | التخفيف | فين بيتختبر |
|---|---|---|
| YAML عدائي (billion laughs، عمق، حجم، مفاتيح مكررة) | الحدود والفحص على `yaml.Node` قبل الـ decode (قرار 8) | fuzz `FuzzParse` واختبارات table |
| حقن أوامر من قيم Git (`${{ git.message }}` في script) | `${{ }}` ممنوع في `run:`، والقيم كمتغيرات بيئة | اختبار بيحط `$(touch /tmp/x)` في رسالة commit |
| تعبير CEL بيستهلك CPU | `CostLimit` ومهلة 10 ms وطول 2 KiB | fuzz `FuzzCEL` وتعبيرات معروفة بالتكلفة |
| webhook مزوّر | HMAC أو التوكن بمقارنة ثابتة الوقت على الـ body الخام قبل الـ parse | اختبار لكل مزوّد: توقيع غلط، وسر مستودع تاني، وbody اتعدّل بايت |
| webhook متعاد | delivery id فريد، والمهلة للصيغة بتاعتنا، وrun واحد لكل (مشروع، حدث، ref، sha) | نفس الـ delivery مرتين ثم بعد ساعة: run واحد |
| سر webhook اتسرّب | التأكد من المزوّد إن الـ sha رأس الـ ref (قرار 11)، والتجديد بزرار | push مزوّر بـ sha قديم على `main`: `superseded` ومفيش job |
| SSRF من عنوان GitLab أو host key أو clone URL | `netguard` في السيرفر، والـ egress proxy في الـ runner (الشبكات الخاصة ممنوعة إلا بإعداد صريح) | `BaseUrl` على `127.0.0.1` و`169.254.169.254` وDNS rebinding: مرفوض |
| حقن argv في git | argv بس و`--` قبل الـ URL والـ ref، والـ refs والـ URLs اتحققوا في `internal/gitref` و`internal/gitsource` | ref بيبدأ بـ `-`، و`--upload-pack=...`، و`ext::sh`: مرفوضين قبل أي process |
| بروتوكولات git خطيرة | `protocol.allow=never` إلا `https` و`ssh`، والـ submodules مقفولة | مستودع فيه submodule بـ `file://`: مابيتجابش |
| تسريب credential الـ checkout | بيتولّد وقت `Start`، ومحصور في المستودع، وفي حاوية الـ checkout بس، وفي الـ masker، ومش في الـ URL | probe في خطوة المستخدم بيدوّر على التوكن في الـ env والملفات و`/proc`: مالقاش |
| أسرار في الـ logs أو الـ outputs | الـ masker بتاع M3 على الـ logs والـ outputs، والـ outputs بتتقص بحد | قيمة التوكن مش في `"LogChunks"` ولا `"RunJobs"."Outputs"` ولا logs السيرفر |
| PR من fork بيغيّر صلاحياته | التعريف من الـ base، ومن غير أسرار ولا بيئات ولا كتابة cache | PR بيضيف egress وenvironment: ماتطبّقوش |
| تسميم الـ cache | الكتابة في scope الفرع بس، والـ fork مابيكتبش | PR بيحاول يحفظ cache الـ `main`: مرفوض |
| artifact بيهرب من الـ workspace | tar حتمي، والفك بيرفض `..` والمسارات المطلقة والروابط لبره والـ devices | artifact مصنوع فيه `../../etc/passwd`: مرفوض |
| سرقة installation على GitHub | التأكد بتوكن المستخدم من `GET /user/installations`، وinstallation لمؤسسة واحدة | مؤسسة ب بتحاول تربط id مؤسسة أ: مرفوض |
| مؤسسة بتشغّل على runners مؤسسة تانية | الـ jobs بالمؤسسة زي M3، والـ run بيورّث مؤسسة المشروع | اختبار العزل بين المؤسسات بيتوسّع للـ runs |
| plugin بيوسّع صلاحياته | نفس التقوية، والـ egress بتاع الـ job، والـ manifest للمدخلات بس | الـ escape suite بيشتغل كـ plugin |

**الأسرار للكتابة بس (اختبار):** اختبار بيمشي على كل الـ DTOs الجديدة ويتأكد إن مفيش مفتاح App ولا توكن ولا سر webhook ولا نص خاص لـ deploy key. والـ access log مابيسجّلش headers الـ webhooks ولا `Authorization`.

## قاعدة البيانات
كل الجداول بالأعمدة الأربعة، والوقت `timestamptz`، والنص `TEXT`، والحذف soft (ما عدا partitions الـ deliveries زي الـ logs)، ومفيش `DELETE FROM` في `Queries/`. وأرقام الـ migrations الرقم الفاضي وقت الـ merge (آخر رقم حاليًا `00040`).

| الجدول | الموديول | أهم الأعمدة والقيود |
|---|---|---|
| `"Projects"` | Projects | `"OrganizationId"` و`"Slug"` (unique مع المؤسسة WHERE NOT deleted) و`"Name"` و`"PipelinePath"` (افتراضيًا `.dhow.yml`) و`"ForkPolicy"` و`"State"` (`active` أو `archived`) |
| `"GitHubApps"` | Sources | `"AppId"` و`"Slug"` و`"ClientId"` و`"PrivateKeySealed"` و`"WebhookSecretSealed"` و`"ClientSecretSealed"`، وواحد بس `active` (partial unique) |
| `"GitConnections"` | Sources | `"OrganizationId"` و`"Provider"` IN (`github`, `gitlab`, `generic`) و`"InstallationId"` (unique WHERE github) و`"BaseUrl"` و`"TokenSealed"` و`"TokenExpiresAt"` و`"State"` |
| `"Repositories"` | Sources | `"ProjectId"` (unique WHERE NOT deleted) و`"ConnectionId"` و`"ExternalId"` و`"FullName"` و`"CloneUrl"` و`"DefaultBranch"` و`"HostKey"` و`"DeployKeyPublic"` و`"DeployKeySealed"` و`"WebhookSecretSealed"` و`"HookId"` (unique) |
| `"GitSetupFlows"` | Sources | `"Kind"` (`manifest` أو `install`) و`"StateHash"` (unique) و`"UserId"` و`"OrganizationId"` و`"ExpiresAt"`، زي flows الـ OIDC |
| `"WebhookDeliveries"` | Sources | `PARTITION BY RANGE ("CreatedAt")` يومي، و`"Provider"` و`"DeliveryId"` (unique مع المزوّد في الـ partition، والـ dedupe بيدوّر في آخر 3 أيام) و`"RepositoryId"` و`"Event"` و`"Outcome"`، والاحتفاظ 30 يوم بـ `DROP` |
| `"CommitStatusReports"` | Sources | `"RepositoryId"` و`"Sha"` و`"RunId"` و`"DesiredState"` و`"SentState"` و`"Attempts"` و`"NextAttemptAt"`، والـ worker بـ `SKIP LOCKED` |
| `"PipelineRuns"` | Pipelines | `"ProjectId"` و`"Number"` (unique مع المشروع) و`"Trigger"` و`"Ref"` و`"Sha"` و`"BaseSha"` و`"IsFork"` و`"PrNumber"` و`"DefinitionSha"` و`"DefinitionDigest"` و`"Definition"` (jsonb بعد التحقق) و`"State"` و`"FailureReason"` و`"ConcurrencyGroup"` و`"DeliveryId"` (unique WHERE NOT NULL) و`"TriggeredByUserId"`، وCHECK على الحالة وtrigger للانتقالات زي `"Jobs"` |
| `"PipelineRunCounters"` | Pipelines | `"ProjectId"` (unique) و`"Next"` |
| `"RunJobs"` | Pipelines | `"RunId"` و`"Key"` (unique مع الـ run) و`"JobName"` و`"Matrix"` (jsonb) و`"Needs"` (`TEXT[]`) و`"State"` و`"CurrentJobId"` و`"Outputs"` (jsonb، 64 KiB بالكتير) و`"FailureReason"` |
| `"Blobs"` | Artifacts | `"Digest"` (bytea 32، unique) و`"SizeBytes"` و`"LastUsedAt"` و`"StorageKey"` |
| `"Artifacts"` | Artifacts | `"OrganizationId"` و`"ProjectId"` و`"RunId"` و`"RunJobKey"` و`"Name"` (unique مع الـ run) و`"Digest"` و`"SizeBytes"` و`"Untrusted"` و`"ExpiresAt"` |
| `"CacheEntries"` | Artifacts | `"ProjectId"` و`"Scope"` و`"Key"` (unique التلاتة) و`"Digest"` و`"SizeBytes"` و`"LastUsedAt"` |

- **تعديل `"Jobs"`** (migration جديد، مش تعديل `00031`): `"OriginKind"` IN (`manual`, `pipeline`) و`"OriginId"` و`"RequiredFeatures"` (`TEXT[]`).
- **`"ApiTokens"`:** migration بيوسّع الـ CHECK على `"Scopes"` بالإجراءات الجديدة.
- **`"RoleBindings"`:** الـ `"ProjectId"` موجود من M2 من غير FK. الـ FK مابيتعملش بين موديولين، والتحقق في الـ service.
- **الـ hook:** قاعدة الأعمدة الأساسية بتقبل `"WebhookDeliveries"` المقسّم بنفس استثناء `"LogChunks"`.

## الـ API

### REST (`packages/openapi/openapi.yaml`، والـ tags `projects` و`sources` و`pipelines` و`artifacts`)
| Route | الإجراء | ملاحظة |
|---|---|---|
| `GET /instance/git-providers/github` | `instance.settings.manage` | الحالة والـ slug، من غير أي سر |
| `POST /instance/git-providers/github/manifest` | `instance.settings.manage` | بيرجّع رابط GitHub والـ manifest |
| `POST /instance/git-providers/github/manifest-callback` | `instance.settings.manage` | `Code` و`State` |
| `POST /instance/git-providers/github/remove` | `instance.settings.manage` | بيقفل كل الـ installations |
| `GET /organizations/{id}/git-connections` | `project.read` | |
| `POST /organizations/{id}/git-connections` | `git.connect` | GitLab: `BaseUrl` و`Token` (للكتابة بس) |
| `POST /organizations/{id}/git-connections/github/install` | `git.connect` | رابط التنصيب بـ `state` |
| `POST /organizations/{id}/git-connections/github/install-callback` | `git.connect` | `InstallationId` و`Code` و`State` |
| `POST /organizations/{id}/git-connections/{connectionId}/revoke` | `git.connect` | |
| `GET /organizations/{id}/git-connections/{connectionId}/repositories` | `project.manage` | للاختيار وقت الربط |
| `GET` و`POST /organizations/{id}/projects` | `project.read` و`project.manage` | |
| `GET` و`PATCH /organizations/{id}/projects/{projectId}` | `project.read` و`project.manage` | `PATCH` للاسم و`PipelinePath` و`ForkPolicy` |
| `POST /organizations/{id}/projects/{projectId}/archive` | `project.manage` | |
| `PUT /organizations/{id}/projects/{projectId}/repository` | `project.manage` | الربط، وبيرجّع النص العام للمفتاح وبصمة الـ host |
| `POST /organizations/{id}/projects/{projectId}/repository/confirm-host-key` | `project.manage` | البصمة اللي اتعرضت |
| `POST /organizations/{id}/projects/{projectId}/repository/rotate-deploy-key` | `project.manage` | النص العام بس |
| `POST /organizations/{id}/projects/{projectId}/repository/rotate-webhook-secret` | `project.manage` | السر بيرجع مرة واحدة |
| `GET /organizations/{id}/projects/{projectId}/runs` | `run.read` | فلتر بالحالة والفرع |
| `POST /organizations/{id}/projects/{projectId}/runs` | `run.trigger` | manual: `Ref` و`Inputs` |
| `GET /organizations/{id}/projects/{projectId}/runs/{runId}` | `run.read` | الـ `RunJobs` والـ jobs والأخطاء |
| `POST /organizations/{id}/projects/{projectId}/runs/{runId}/cancel` | `run.cancel` | idempotent |
| `POST /organizations/{id}/projects/{projectId}/runs/{runId}/rerun-failed` | `run.trigger` | |
| `POST /organizations/{id}/projects/{projectId}/runs/{runId}/approve` | `run.approve` | PR من fork |
| `GET /organizations/{id}/projects/{projectId}/runs/{runId}/artifacts` | `artifact.read` | |
| `GET /organizations/{id}/projects/{projectId}/artifacts/{artifactId}/download` | `artifact.read` | stream، و`Content-Disposition: attachment` |

- **الـ callbacks:** المزوّد بيرجع لصفحة في الواجهة، والواجهة بتشيل `code` و`state` من العنوان وتبعتهم مرة واحدة بـ `POST`، زي الـ OIDC.
- **`POST /organizations/{id}/jobs`** (أداة M3): حسب [السؤال 2](#2-مصير-post-jobs).

**الأدوار المقترحة** (الصلاحية على مستوى المشروع بتيجي من `"RoleBindings"` لو موجودة، وغير كده من دور المؤسسة):

| الإجراء | Owner | Admin | Maintainer | Developer | Viewer |
|---|:-:|:-:|:-:|:-:|:-:|
| `project.read` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `project.manage` | ✅ | ✅ | ✅ | — | — |
| `git.connect` | ✅ | ✅ | — | — | — |
| `run.read` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `run.trigger` | ✅ | ✅ | ✅ | ✅ | — |
| `run.cancel` | ✅ | ✅ | ✅ | — | — |
| `run.approve` | ✅ | ✅ | ✅ | — | — |
| `artifact.read` | ✅ | ✅ | ✅ | ✅ | — |

### الـ webhooks (بره `/api/v1`)
| Route | المصادقة | ملاحظة |
|---|---|---|
| `POST /webhooks/github` | `X-Hub-Signature-256` بسر الـ App | الـ installation والمستودع لازم مربوطين |
| `POST /webhooks/gitlab/{hookId}` | `X-Gitlab-Token` لكل مستودع | التأكد بالـ API إجباري |
| `POST /webhooks/git/{hookId}` | HMAC لكل مستودع (Gitea أو Forgejo أو الصيغة بتاعتنا) | push وtag بس |

- **الـ middleware:** RequestID وClientIP وAccessLog (من غير headers التوقيع) وRecover وBodyLimit وRateLimit. ومن غير جلسة ولا CSRF.
- **الـ hook:** `check_route_policies` بيقبل `policy.Webhook(...)` في `Sources.Routes.go` بس، ومعاه اختبارات.

### الـ proto (`packages/proto/dhow/runner/v1`، إضافات بس، و`buf breaking` أخضر)
```
JobSpec   += checkout Checkout, artifacts[] ArtifactUpload, inputs[] ArtifactInput, cache Cache
Step      += id, plugin Plugin { image (digest), inputs map<string,string> }
Checkout  { repository_url, ref, sha, transport (https | ssh), credential_secret, known_host,
            depth, egress[] }
ArtifactUpload { name, paths[], max_bytes }
ArtifactInput  { name, digest, size_bytes, download_token }
Cache     { key_expression, paths[], scope, restore_scopes[], read_only, max_bytes }

LeaseRequest    += features[]          (checkout, plugins, artifacts, cache)
CompleteRequest += outputs map<string,string>   (masked, capped)

ArtifactService.Commit(LeaseRef, name, digest, size_bytes) → ok    (after the PUT below)
CacheService.Lookup(LeaseRef, key) → hit, digest, size_bytes, download_token
CacheService.Commit(LeaseRef, key, digest, size_bytes) → ok

raw HTTP on /runner-api (runner access token + Dhow-Lease-Id + Dhow-Fencing-Token):
  PUT /runner-api/blobs/{sha256}   body = tar.gz, size cap, digest checked while writing
  GET /runner-api/blobs/{sha256}   Dhow-Download-Token bound to (lease, digest), lease must be active
```
- **التوافق:** السيرفر مابيدّيش job فيه `"RequiredFeatures"` لـ runner ما أعلنهاش في `LeaseRequest.features`. فالـ runner القديم بيكمّل يشغّل jobs بتاعة M3، والـ job الجديد بيستنى بـ `NO_MATCHING_RUNNER` ظاهر.
- **بروتوكول الـ agent:** frame جديد `RunExec` (argv وenv)، مسموح بس للخطوات اللي الـ runner بيقرّرها (الـ checkout والـ plugin)، عمره ما بيتبني من نص الـ job.

### الإعدادات الجديدة
- **السيرفر** (بتتضاف لـ `docs/install/configuration.md`، واللي ينفع منها بيبقى في «إعدادات السيرفر» حسب ADR-0063):
  - `DHOW_BLOB_DIR` (افتراضيًا `/var/lib/dhow/blobs`، والإقلاع بيقف لو مش موجود أو صلاحياته مفتوحة).
  - `DHOW_ARTIFACT_MAX_BYTES` و`DHOW_ARTIFACT_RETENTION_DAYS` و`DHOW_CACHE_MAX_BYTES_PER_PROJECT`.
  - `DHOW_WEBHOOK_MAX_BYTES` و`DHOW_GIT_ALLOW_PRIVATE_CIDRS`.
  - `DHOW_PUBLIC_URL` بقى إجباري لو فيه أي مزوّد متوصّل، لأن الـ webhooks والـ callbacks بتتبني منه.
- **الـ runner:** `DHOW_RUNNER_CHECKOUT_IMAGE` (افتراضيًا الـ digest المتثبّت في الـ build، والتغيير بالـ digest بس).

## تقسيم الـ PRs
كل PR بيعدّي `make check` وCI أخضر، وفيه اختبارات سلبية لكل مسار أمني، وبيحدّث `Plan/Progress/` في يومه، وبيكتب الـ ADR بتاع قراراته في أوله. والأوامر بتفترض نفس `DHOW_TEST_DATABASE_URL` بتاع M3.

### M4.1: `internal/pipelinespec`: الـ parser والـ validator وCEL والـ DAG
- **الاعتماد:** M3.13.
- **المحتوى:**
  - `internal/pipelinespec/{parse,validate,cel,dag,matrix,compile}.go`: من الملف لـ «خطة» فيها الـ `RunJobs` والـ `JobSpec` لكل واحد.
  - `packages/spec/dhow.schema.json` واختبار التطابق (قرار 9).
  - `go.mod`: `github.com/google/cel-go v0.30.0`.
  - `docs/pipeline-spec.md` و`docs/en/pipeline-spec.md`: `resources`، وقيود `run:`، والحدود، والـ triggers المؤجلة.
  - ADR للقرارات 7 و8 و9 و13.
- **قبول:**
  - كل مثال في `docs/pipeline-spec.md` بيعدّي، وفيه اختبار بيقرا الأمثلة من الوثيقة نفسها.
  - اختبارات table لكل خطأ: دورة، وneed مش موجود، ومفتاح مش معروف، وalias، وtag، ومفتاح مكرر، و`${{ }}` في `run:`، و`secrets` في `if`، وmatrix أكبر من 256، وصورة `uses:` من غير digest.
  - CEL: نوع غلط بيترفض وقت الـ validation، وتعبير مكلف بيقف عند الحد في أقل من 50 ms.
  - التوسيع حتمي: نفس الملف بيطلّع نفس الـ digest لكل `JobSpec`.
- **التحقق:**
  - `go test -race -count=1 ./internal/pipelinespec/...`
  - `go test -run '^$' -fuzz FuzzParse -fuzztime 60s ./internal/pipelinespec` و`FuzzCEL` و`FuzzMatrix`.
  - `make docs-check` و`make check`.

### M4.2: `dhow lint`
- **الاعتماد:** M4.1.
- **المحتوى:**
  - `apps/cli` بالهيكل اللي فوق، والـ hook بيتوسّع لـ `apps/cli/internal` (PascalCase واتجاه الاستيراد) ومعاه اختبارات.
  - الرسايل بالعربي والإنجليزي من catalog (`*Ar.go`) وبيعدّي hook العربي المبسّط.
  - `make build-cli`، والأرشيفات في `release.yml`، و`get.sh --component cli`.
  - `docs/cli.md` بالنسختين.
- **قبول:**
  - نفس الملفات الغلط في اختبارات M4.1 بتطلّع نفس الأكواد من الـ CLI.
  - `--format json` ثابت ومعاه golden files.
  - مفيش أي اتصال شبكة (اختبار بيشغّله في namespace من غير شبكة).
- **التحقق:** `make build-cli` و`go test -race -count=1 ./apps/cli/...` و`make hooks-test` و`make check`.

### M4.3: موديول `Projects` والـ ports والصلاحيات على المشروع
- **الاعتماد:** M3.13.
- **المحتوى:**
  - الـ migration لـ `"Projects"`، وتعديل `"Jobs"` (`"OriginKind"` و`"OriginId"` و`"RequiredFeatures"`)، وتوسيع scopes التوكنات.
  - routes المشاريع، والإجراءات الجديدة في `Core/Policy`، و`policy.Project(...)` اللي بيحل الدور من `"RoleBindings"` على المشروع.
  - الـ ports في `Core` (قرار 1) بـ interfaces بس، وتنفيذ `Core/JobQueue` في `Jobs`، والتركيب في `App`.
  - الـ hook: اتجاه الاستيراد للـ ports، ومعاه اختبارات.
  - ADR للقرار 1.
- **قبول:**
  - اختبار الـ route-table فيه صف لكل route ولكل دور، ومعاه binding على المشروع بيرفع أو بيقلّل الدور.
  - `JobQueue.Enqueue` جوه transaction اتعمله rollback: مفيش job.
  - Viewer مايقدرش يعمل مشروع، وDeveloper بـ binding Maintainer على مشروع يقدر يعدّله هو بس.
- **التحقق:** `go test -race -count=1 ./apps/server/internal/Modules/Projects/... ./apps/server/internal/Core/... ./apps/server/internal/Modules/Jobs/...` و`make gen && git diff --exit-code` و`make hooks-test` و`make check`.

### M4.4: موديول `Sources`: الطبقة المشتركة وGit عام
- **الاعتماد:** M4.3.
- **المحتوى:**
  - `internal/gitref` و`internal/gitsource` و`internal/netguard` (والـ runner بيستخدم `netguard` بدل `Egress.Ranges.go`).
  - الـ migrations لـ `"GitConnections"` و`"Repositories"` و`"GitSetupFlows"` و`"WebhookDeliveries"` و`"CommitStatusReports"`.
  - `/webhooks/` في `App` بالـ middleware بتاعته، و`policy.Webhook`، والـ hook.
  - توليد الـ deploy key، وجلب الـ host key وتأكيده، وwebhook Git العام بالتوقيعات الأربعة.
  - الـ dedupe، والـ `Trigger` الموحّد، والتسليم بـ `Core/Triggers`.
  - صيانة partitions الـ deliveries.
  - الـ worker بتاع `"CommitStatusReports"` (فاضي لحد M4.5).
  - ADR للقرارات 11 و12 و15.
- **قبول:**
  - **التوقيع:** HMAC غلط، وسر مستودع تاني، وbody اتعدّل، و`t` أقدم من 5 دقايق: `401` بنفس الرد ومن غير صف delivery.
  - **التكرار:** نفس الـ delivery مرتين: trigger واحد.
  - **SSRF:** host key أو clone URL على loopback وprivate وmetadata، وDNS rebinding، وredirect: مرفوضين، وبيعدّوا لما النطاق في `DHOW_GIT_ALLOW_PRIVATE_CIDRS`.
  - **الـ URLs والـ refs:** table فيه `ext::`، و`file://`، و`-oProxyCommand`، و`--upload-pack`، وحروف تحكم، و`..`، و`@{`: مرفوضين.
  - **الأسرار:** النص الخاص للمفتاح وسر الـ webhook مش في أي response بعد أول مرة، ولا في الـ access log، ولا في الـ audit.
- **التحقق:**
  - `go test -race -count=1 ./internal/gitref/... ./internal/gitsource/... ./internal/netguard/... ./apps/server/internal/Modules/Sources/...`
  - `go test -run '^$' -fuzz FuzzWebhook -fuzztime 60s ./apps/server/internal/Modules/Sources/Test/Edge` و`FuzzRef` و`FuzzCloneUrl`.
  - `make hooks-test` و`make check`.

### M4.5: GitHub App
- **الاعتماد:** M4.4.
- **المحتوى:**
  - الـ migration لـ `"GitHubApps"`.
  - الـ manifest flow، وربط الـ installation بتوكن المستخدم، والـ JWT، وتوكنات الـ installation.
  - قراية التعريف، والـ webhooks (push وpull_request وcheck_run وinstallation)، والـ check runs.
  - توليد credential الـ checkout وقت `Start` من خلال `SecretResolver` (اسم محجوز `dhow.git.checkout`).
  - ADR للقرارات 14 و16.
- **قبول:**
  - **الـ API:** سيرفر GitHub وهمي في الاختبار (`httptest`) بيغطي التحويلات والأخطاء والـ rate limit.
  - **السرقة:** ربط installation مش في `GET /user/installations`: مرفوض. وinstallation مربوط بمؤسسة: مايتربطش بتانية.
  - **التوكنات:** توكن الـ checkout محصور في المستودع و`contents: read` (بنتأكد من الطلب)، ومش في قاعدة البيانات (`pg_dump` مافيهوش).
  - **الـ check:** الملخص من غير logs.
  - **E2E حقيقي** (ليلي وعلى label `providers`): مستودع اختبار بيعمل push، والـ run بيشتغل، والـ check بيبقى أخضر.
- **التحقق:** `go test -race -count=1 ./apps/server/internal/Modules/Sources/...` و`make check`، وjob `providers` في الـ CI.

### M4.6: GitLab
- **الاعتماد:** M4.4.
- **المحتوى:** الاتصال بالتوكن والتحقق منه، وربط المشروع (webhook وdeploy key بالـ API أو يدوي)، والـ webhook بالتوكن والتأكد بالـ API، وقراية التعريف، والـ commit statuses.
- **قبول:**
  - توكن ناقص scope: الحفظ مرفوض برسالة واضحة.
  - `X-Gitlab-Token` غلط: `401`. وتوكن صح بـ sha مش رأس الـ ref: `superseded`.
  - توكن الـ API مش موجود في أي `JobSpec` ولا `SealedSecrets` (اختبار بيفك الـ bundle في الاختبار ويدوّر).
  - **E2E حقيقي** على GitLab.com (ليلي)، وعلى GitLab CE في حاوية (ليلي، لأنه تقيل).
- **التحقق:** `go test -race -count=1 ./apps/server/internal/Modules/Sources/...` و`make check`، وjob `providers`.

### M4.7: الـ runner: أكتر من حاوية، والـ checkout، والإضافات
- **الاعتماد:** M3.13 (والـ proto ممكن يتعمل بالتوازي مع M4.1).
- **المحتوى:**
  - **الـ spike (go/no-go):** workspace مشترك بين حاويات متتابعة بنفس الـ uid map، بطريقتين: pod بـ `--userns=auto` مشترك، أو uid map صريح من نطاق الـ runner مع tmpfs volume. شروط القبول: الـ escape والـ limits suites خضرا على الاتنين، وحد القرص بيتحسب صح، ومفيش volume فاضل بعد الهدم. والنتيجة في ADR.
  - **إضافات الـ proto** والـ `features`، و`RunExec` في بروتوكول الـ agent.
  - **صورة `dhow-git`:** بتتبني في `release.yml` من base متثبّت فيها `git` و`openssh-client` بس، وبتتوقّع بـ cosign، والـ digest بيتكتب في الـ runner وقت الـ build.
  - **الـ checkout** بكل الإعدادات اللي فوق، والـ helpers `askpass` و`ssh`.
  - **الإضافات:** قراية الـ manifest، والتحقق من `with:`، والتشغيل بـ argv، والـ outputs.
  - **`internal/jobspec`** بيتحقق من الحقول الجديدة.
  - ADR للقرارات 3 و4 و5.
- **قبول:**
  - **الـ escape suite** بيشتغل كخطوة `run` وكـ plugin، وبيتوسّع:
    - خطوة المستخدم مش شايفة credential الـ checkout (env وملفات و`/proc`).
    - خطوة المستخدم ماتقدرش توصل لـ host المستودع إلا لو في الـ egress بتاعها.
    - ملفات الـ checkout مافيهاش setuid ولا devices (الـ tmpfs بـ `nosuid,nodev`).
  - **الـ checkout:** ref اتحرك: fetch بالـ sha أو `CHECKOUT_REF_MOVED`. وhost key مختلف: `HOST_KEY_MISMATCH`. ومستودع فيه submodule بـ `file://` أو hook: مفيش حاجة اتنفّذت.
  - **الإضافات:** `with:` مش مطابق للـ manifest: `PLUGIN_INPUT_INVALID` قبل التشغيل. وصورة من غير `/bin/sh` بتشتغل.
  - **التوافق:** runner بنسخة M3 مابياخدش job فيه `checkout`.
- **التحقق:** `go test -race -count=1 -tags dhowtest ./apps/runner/...` و`make runner-isolation` و`make proto-lint` و`make check`، وjob `isolation` في الـ CI.

### M4.8: موديول `Pipelines`: الـ runs والـ DAG والـ triggers
- **الاعتماد:** M4.1 وM4.4 وM4.7 (وM4.5 وM4.6 للمزوّدين دول).
- **المحتوى:**
  - الـ migrations لـ `"PipelineRuns"` و`"PipelineRunCounters"` و`"RunJobs"`، والـ triggers على الحالات.
  - تنفيذ `Core/Triggers` و`Core/RunEvents`، والتوسيع، والإدراج، والتقييم بعد كل job، والإلغاء، و«أعد تشغيل الفاشل».
  - الـ discovery job لـ Git عام.
  - الـ concurrency groups (advisory lock على المشروع والمجموعة، و`cancel_in_progress`).
  - سياسة الـ forks والموافقة.
  - الـ manual وapi triggers بالـ inputs.
  - الـ reconciler.
  - ADR للقرارين 2 و10، وقرار سياسة الـ forks بعد إجابة السؤال 1.
- **قبول:**
  - **الـ DAG:** pipeline بـ 4 jobs (اتنين بالتوازي وmatrix 2×2) بيخلص بالترتيب الصح، والـ need الفاشل بيعمل `skipped` للي بعده.
  - **مرة واحدة:** نفس الـ trigger مرتين (delivery مختلف ونفس الـ sha والـ ref): run واحد. و100 حدث `JobFinished` متوازي على نفس الـ run: كل `RunJob` بيتدرج مرة واحدة بس.
  - **الإلغاء:** إلغاء run فيه job شغّال بيوصل الـ runner في الـ heartbeat الجاي، والـ `waiting` بيبقوا `canceled`.
  - **الـ concurrency:** run جديد بـ `cancel_in_progress` بيلغي القديم، ومن غيره بيستنى.
  - **الـ forks:** PR من fork بيعدّل `.dhow.yml`: التعريف اللي اتطبّق من الـ base (الـ digest بيثبت). ومفيش أسرار ولا بيئات ولا كتابة cache.
  - **التعريف الغلط:** `PIPELINE_INVALID` بالأخطاء في الـ API وفي الـ check.
  - **الأعطال:** قتل السيرفر بعد ما الـ job خلص وقبل الـ commit: بعد القيام، الـ run بيكمّل صح (مفيش حدث ضايع لأن الكل في transaction واحدة).
- **التحقق:**
  - `go test -race -count=1 ./apps/server/internal/Modules/Pipelines/...`
  - `go test -race -count=20 -run 'Race|Once' ./apps/server/internal/Modules/Pipelines/Test/Edge/...`
  - `make check`.

### M4.9: الـ artifacts والـ cache
- **الاعتماد:** M4.7 وM4.8.
- **المحتوى:**
  - موديول `Artifacts`: الـ migrations، و`Core/Blob` بـ driver القرص (كتابة لملف مؤقت ثم `rename` بعد تأكيد الـ digest، والمسار من الـ digest بس).
  - الـ endpoints الخام على `/runner-api/blobs/` بالـ gate بتاع الـ runner والـ lease، و`ArtifactService` و`CacheService`.
  - تنفيذ `Core/ArtifactIndex` لـ `Pipelines`.
  - الرفع والتنزيل والفك في الـ runner.
  - الاحتفاظ والتنضيف (mark and sweep على `"Blobs"` تحت advisory lock).
  - احتفاظ الـ logs لكل مؤسسة (البند المتأجل من M3).
  - الـ hooks: `Core/RunnerAuth` بيتستورد من `Modules/Artifacts`، وتركيب الـ Connect handlers في `Artifacts.Routes.go`.
  - ADR للقرار 6.
- **قبول:**
  - **الـ artifact:** بيعدّي من `build` لـ `deploy-check` بنفس الـ digest. وdigest غلط في الرفع بيترفض ومابيتخزّنش. وtar فيه `..` أو رابط لبره أو device بيترفض.
  - **الـ lease:** رفع أو تنزيل بـ lease خلص أو fencing قديم: `LEASE_LOST`. وتنزيل blob مش في الـ grant: مرفوض.
  - **الـ cache:** بيتسترجع في التشغيل التاني. وPR من fork مابيكتبش. والحد بيمسح الأقدم.
  - **الحدود:** artifact أكبر من الحد بيقف في نص الرفع من غير ما يملا القرص.
  - **التنضيف:** blob مالوش artifact ولا cache بيتمسح بعد المهلة، والمستخدم لسه مابيتمسحش.
- **التحقق:** `go test -race -count=1 ./apps/server/internal/Modules/Artifacts/... ./apps/runner/...` و`make runner-isolation` و`make check`.

### M4.10: الواجهة: المزوّدين والمشاريع والـ runs
- **الاعتماد:** M4.5 وM4.8 (وM4.9 للـ artifacts).
- **المحتوى:**
  - **إعدادات السيرفر:** «اعمل GitHub App» وحالته.
  - **المؤسسة:** «اربط GitHub» و«اربط GitLab».
  - **المشاريع:** إنشاء، وربط مستودع بالخطوات (المفتاح العام، وتأكيد البصمة، والسر مرة واحدة)، وإعدادات الـ forks.
  - **الـ runs:** قايمة، وتفاصيل فيها الـ DAG كقايمة مرتّبة بالمراحل (مش رسم، الرسم في M7)، والـ job بيفتح الـ log الحي الموجود من M3.12، وأزرار الإلغاء والإعادة والموافقة.
  - **النصوص:** مصطلحات جديدة في `Plan/Glossary.md` (Pipeline وRun وProject وDeploy key وWebhook وArtifact وCache)، والمفاتيح في `Locales/Workspace/{Ar,En}.ts`.
- **قبول:**
  - lint وtypecheck والاختبارات والـ build والـ size في الميزانية.
  - Playwright على سيرفر حقيقي وForgejo في حاوية: ربط مستودع، وpush، وrun أخضر، بالعربي والإنجليزي وموبايل 390px، ومفيش خطأ في الـ console ولا مخالفة CSP.
  - السر والـ code والـ state مش في الـ URL بعد الـ callback ولا في الـ history ولا `localStorage`.
- **التحقق:** `pnpm -C apps/web lint && pnpm -C apps/web typecheck && pnpm -C apps/web test && pnpm -C apps/web build && pnpm -C apps/web size`، و`make web` و`make check`.

### M4.11: التقوية وإثبات شرط الخلاص
- **الاعتماد:** كل اللي فات.
- **المحتوى:**
  - fuzz طويل ليلي للـ targets كلها (30 دقيقة لكل واحد).
  - مراجعة تهديدات للـ pipelines والمزوّدين في `docs/security-model.md` (بالنسختين).
  - مراجعة أمنية مستقلة، وإصلاح الـ Critical والـ High.
  - `deploy/m4-demo/run.sh`: Forgejo في حاوية، وسيرفر، وrunner بـ Podman، ومستودع فيه pipeline بـ DAG وmatrix وartifact وcache، وwebhook مزوّر ومتعاد، وبيطبع النتيجة.
  - `docs/roadmap.md` بالنسختين: M4 خلص.
- **قبول:**
  - الـ CI أخضر: `isolation` و`chaos` و`providers`.
  - `deploy/m4-demo/run.sh` نجح على VM نضيفة بـ Debian 13 وUbuntu 26.04.
  - التلات مزوّدين الحقيقيين شغّالين في job `providers` آخر 3 ليالي.
  - `grep` لكل credential (مفتاح الـ App، وتوكن GitLab، والـ deploy keys، وأسرار الـ webhooks، وتوكنات الـ checkout) في `pg_dump` وlogs السيرفر والـ runner وتخزين Podman والـ journal مالقاش حاجة صريحة.
- **التحقق:** `make check` و`make runner-isolation` و`make runner-chaos` و`deploy/m4-demo/run.sh`.

### الترتيب
```
M3.13 ─┬─► M4.1 ─┬─► M4.2
       │         └───────────────────────────┐
       ├─► M4.3 ─► M4.4 ─┬─► M4.5 ──────┐    │
       │                 ├─► M4.6       │    ▼
       │                 └────────────► M4.8 ─► M4.9 ─► M4.10
       └─► M4.7 ─────────────────────────▲
                                                   M4.11 في الآخر
```
- M4.1 وM4.3 وM4.7 بيبدأوا مع بعض بعد M3.13.
- M4.8 بيبدأ بـ Git عام (M4.4)، وGitHub وGitLab بيتوصّلوا لما M4.5 وM4.6 يخلصوا.
- M4.10 محتاج M4.5 للـ GitHub App.

## الاختبارات: إيه بيشتغل فين
| الاختبار | كل PR | ليلي أو label | قبل كل release |
|---|---|---|---|
| `internal/pipelinespec` (table وأمثلة الوثيقة) وfuzz قصير | ✅ | — | — |
| `dhow lint` (golden files، من غير شبكة) | ✅ | — | — |
| `Sources` و`Pipelines` و`Artifacts` على Postgres 18 (route-table والانتقالات والسباقات) | ✅ | — | — |
| المزوّدين بسيرفرات وهمية (`httptest`) لـ GitHub وGitLab وGitea | ✅ | — | — |
| Forgejo في حاوية متثبّتة بالـ digest: ربط وwebhook وcheckout وrun كامل | ✅ لو الـ PR بيلمس `Sources` أو `Pipelines` أو `apps/runner` | ✅ | ✅ |
| escape وlimits مع الـ checkout والإضافات على Podman rootless (`ubuntu-26.04`) | ✅ لو الـ PR بيلمس `apps/runner` أو `packages/proto` | ✅ | ✅ VM بـ Debian 13 وUbuntu 26.04 |
| GitHub حقيقي (App اختبار، ومستودع، وfork من حساب تاني) | — | ✅ ليلي وعلى label `providers` | ✅ |
| GitLab.com حقيقي | — | ✅ ليلي وعلى label `providers` | ✅ |
| GitLab CE في حاوية (self-managed) | — | ✅ ليلي | ✅ |
| الفوضى (chaos) مع الـ runs: قتل السيرفر بين `Complete` والـ commit | — | ✅ ليلي وعلى label `chaos` | ✅ |
| fuzz طويل (YAML وCEL والـ webhooks والـ refs والـ URLs والـ tar) | — | ✅ ليلي (30 دقيقة لكل target) | — |
| Playwright للواجهة (سيرفر حقيقي وForgejo) | ✅ لو الـ PR بيلمس `apps/web` | — | ✅ |

الـ jobs اللي بتلمس مزوّدين حقيقيين بتاخد أسرارها من GitHub environment محمي، ومابتشتغلش على PRs من forks ولا على الـ draft (نفس شرط `!github.event.pull_request.draft`).

## Edge cases
- **force push:** الـ sha القديم ممكن يختفي. الـ run اللي اتعمل له بيفشل `CHECKOUT_REF_MOVED` لو اتشال، والـ run الجديد بيتعمل من الـ push الجديد.
- **push بأكتر من commit:** run واحد على الـ `after` بس.
- **مسح فرع (`after` كله أصفار):** مابيعملش run.
- **tag خفيف ومُعلّق (annotated):** الاتنين بيتحلّوا للـ commit (`^{commit}`)، والـ run على الـ commit.
- **`.dhow.yml` مش موجود:** مفيش run ومفيش check (عشان المستودعات اللي متربطتش لسه مايبقاش فيها checks حمرا)، وسطر في صفحة المشروع.
- **`PipelinePath` اتغيّر في PR من fork:** الإعداد على المشروع في داو مش في المستودع، فمايتأثرش.
- **المستودع اتنقل أو اتغيّر اسمه:** GitHub وGitLab بالـ id مش بالاسم، فالربط بيفضل. وGit عام: الـ webhook بالـ `HookId`، والـ clone URL بيتعدّل بإيد.
- **الـ installation اتشال أو اتعلّق:** المشاريع بتتعلّم `disconnected`، والـ runs الجديدة بتترفض بسبب واضح، والشغّالة بتكمّل لحد ما توكن الـ checkout يخلص (ساعة).
- **توكن GitLab خلص:** نفس اللي فوق، والتحذير قبلها بـ 14 يوم في الواجهة والـ audit (الإيميل مع M7).
- **الـ host key اتغيّر:** `HOST_KEY_MISMATCH` لحد ما أدمن يأكّد. عمرنا ما بنقبله أوتوماتيك.
- **webhook وصل قبل ما الـ commit يبان في الـ API** (تأخير عند المزوّد): التأكد بيعيد 3 مرات في 30 ثانية قبل `superseded`.
- **المزوّد واقع وقت التقرير:** الـ worker بيعيد بـ backoff لحد 24 ساعة، وآخر حالة بس هي اللي بتتبعت.
- **matrix فاضي بعد `exclude`:** خطأ validation (`MATRIX_EMPTY`) مش run فاضي.
- **job كل الـ `needs` بتاعته `skipped` بسبب `if`:** بيعتبرها نجحت (الـ `skipped` بالشرط غير الـ `skipped` بالفشل)، ومكتوب في الـ spec.
- **artifact بنفس الاسم من نسختين matrix:** خطأ validation، والاسم لازم فيه قيمة من الـ matrix.
- **الـ runner القديم:** job فيه `checkout` بيفضل `queued` بـ `NO_MATCHING_RUNNER` ظاهر وبعد `QueueDeadline` بيفشل، والواجهة بتقول «حدّث جهاز التشغيل».
- **السيرفر ورا firewall ومفيش webhook بيوصل:** الـ manual trigger شغّال، والواجهة بتعرض آخر delivery وصل عشان المشكلة تبان.
- **مستودع ضخم:** الـ checkout بـ `--depth=1` افتراضيًا، والقرص بحد الـ job، و`ENOSPC` بيطلع كـ `CHECKOUT_DISK_FULL` واضح.

## المخاطر
| الخطر | التخفيف |
|---|---|
| الـ workspace المشترك بين حاويات مع userns مختلف بيكسر الملكية | spike بشروط في أول M4.7، وطريقتين، والنتيجة في ADR قبل أي كود تاني |
| أعطال وتغييرات في APIs المزوّدين | سيرفرات وهمية في كل PR، والمزوّدين الحقيقيين ليلي، والـ API calls قليلة ومعروفة |
| GitLab CE في الـ CI تقيل (ذاكرة ووقت) | ليلي بس، وGitLab.com هو المرجع في الـ PRs اللي عليها label |
| الـ GitHub App محتاج `DHOW_PUBLIC_URL` يوصله GitHub | مكتوب في الوثائق، و«اختبر الـ webhook» في الواجهة، والـ manual trigger |
| الـ discovery job بيستهلك runner لكل push على Git عام | job صغير (128 MiB، 30 ثانية، shallow)، وأولوية أعلى، وcache للتعريف بالـ sha |
| `cel-go` اعتمادية كبيرة (ANTLR) | مطلوبة بقرار قديم (CEL بس)، ومتثبّتة، وfuzz عليها، وبره الـ runner اللي بيقيّم `hash_files` بس |
| credentials المزوّدين بـ `DHOW_SECRET_KEY` لحد M5 | مكتوب، وM5 بينقلهم، وتغيير المفتاح بيقفل الربط بوضوح مش بصمت |
| الـ webhook مابيكونش ليه توقيع في GitLab | التأكد بالـ API إجباري (قرار 11) |
| أرقام الـ migrations بتتخبط بين المراحل | الرقم وقت الـ merge |
| ميزانية الـ JS في الواجهة | الصفحات lazy، والـ DAG قايمة مش مكتبة رسم |

## الاعتماديات المتوقعة (بنسخ مثبّتة)
اتأكدت يوم 2026-10-30 من proxy.golang.org (التاريخ) وOSV (مفيش advisories معروفة)، وكلها أقدم من أسبوعين.

| الاعتمادية | النسخة | تاريخها | ليه |
|---|---|---|---|
| `github.com/google/cel-go` | v0.30.0 | 2026-07-25 | CEL للشروط والقيم (قرار قديم في `CLAUDE.md`). بتجيب `cel.dev/expr` v0.25.1 و`github.com/antlr4-go/antlr/v4` v4.13.1 |
| `go.yaml.in/yaml/v3` | v3.0.5 (موجودة) | — | الـ parsing لـ `yaml.Node` |
| `golang.org/x/crypto/ssh` | من v0.58.0 (موجودة) | — | صيغة الـ deploy key وجلب الـ host key. GO-2026-5932 لسه متابَع في `Plan/Status.md` |
| صورة Forgejo (CI) | بالـ digest وقت التنفيذ | — | Git عام في الاختبارات |
| صورة GitLab CE (CI ليلي) | بالـ digest وقت التنفيذ | — | GitLab self-managed |
| base صورة `dhow-git` | بالـ digest وقت التنفيذ | — | `git` و`openssh-client` بس |

**مش هنضيف:**
- `go-github` و`ghinstallation` ومكتبة JWT: الـ calls قليلة، وRS256 من الـ standard library.
- مكتبة GitLab: نفس السبب.
- `go-git`: مفيش git في السيرفر (قرار 2).
- مكتبة JSON Schema: الـ validator بـ Go هو المصدر.
- مكتبة glob: الـ patterns بسيطة ومعاها fuzz.
- SDK لـ S3 أو `minio-go`: حسب السؤال 4.
- zstd: الـ gzip من الـ standard library كفاية لأول نسخة.

## أسئلة مفتوحة لـ Sabry

**الإجابات (2026-10-30):** Sabry اختار التوصية في التمانية:
1. PRs من الـ forks بتستنى موافقة عضو (Maintainer أو أعلى)، والمشروع يقدر يختار التشغيل الأوتوماتيك من غير أسرار.
2. `POST /jobs` بيتشال من الإنتاج، ويفضل للاختبارات و`DHOW_DEV`.
3. الـ 5 capabilities إعداد للمشروع يفتحه الأدمن، وعمره ما بيتطبّق على forks، وبـ ADR بيعدّل ADR-0043.
4. الـ artifacts والـ cache على القرص بس في أول نسخة، ورا interface `Core/Blob`.
5. GitHub.com وGitLab.com وGitLab self-managed. وGHES بعد أول نسخة.
6. Git عام: webhook وتشغيل يدوي بس، ومن غير polling.
7. Sabry هيجهّز حسابات الاختبار قبل M4.5، وهنبعتله الخطوات بالظبط ساعتها.
8. GitLab: أي توكن بالـ scopes الصح، والواجهة بتحذّر من الـ personal token.

### 1. سياسة PRs من الـ forks
الكود من حد بره المؤسسة بيشتغل على أجهزة العميل، حتى من غير أسرار.
- **أ. موافقة لكل push من حد مش عضو** (Maintainer أو أعلى بيدوس «وافق وشغّل»): أأمن، وده اللي `docs/security-model.md` بيقوله. بس فيه احتكاك للمشاريع المفتوحة الكبيرة.
- **ب. تشغيل أوتوماتيك من غير أسرار ولا بيئات:** أسهل، بس أي حد يقدر يستهلك الـ runners ويستخدم الـ egress المسموح (تعدين، أو spam).
- **ج. الـ forks مابتشتغلش خالص:** أبسط، بس المشاريع المفتوحة ملهاش CI على الـ PRs.
- **إضافة ممكنة مع أي اختيار:** jobs الـ forks على runners عليها label مخصص بس.
- **التوصية:** (أ) افتراضيًا، والمشروع يقدر يختار (ب) بإعداد من الأدمن. ومن غير label إجباري، لأن الـ sandbox معمول لكود عدائي، بس الإعداد موجود للي عايز يعزل أكتر.

### 2. مصير `POST /jobs`
أداة M3 دي بتخلّي صاحب المؤسسة أو الأدمن يشغّل أي صورة على أي runner من غير مراجعة كود (ADR-0047 قال M4 يقرر).
- **أ. نشيلها من الـ API في الإنتاج** ونسيبها للاختبارات و`DHOW_DEV`، والبديل `on: manual` بـ inputs: كل تشغيل من ملف في المستودع ومراجَع.
- **ب. نسيبها لصاحب المؤسسة والأدمن** كـ «تشغيل يدوي»: مرنة للصيانة، بس باب بيعدّي كل سياسات المشروع.
- **التوصية:** (أ)، لأن أي حاجة بتشتغل على أجهزة العميل لازم يبقى ليها مصدر في Git ومراجعة.

### 3. الـ capabilities جوه الـ job (`apt-get`)
M3 شال كل الـ capabilities، فـ `apt-get install` جوه الـ job بيفشل (اتأجل لـ M4 بـ ADR).
- **أ. نسيبها صفر:** أأمن، والمستخدم يجهّز صورة فيها اللي محتاجه. بس تجربة أول مرة صعبة، وناس هتسأل.
- **ب. إعداد للمشروع (الأدمن بس)** بيدّي 5 capabilities جوه الـ user namespace (`CHOWN` و`DAC_OVERRIDE` و`FOWNER` و`SETUID` و`SETGID`)، وعمره ما بيتطبّق على PRs من forks: `apt-get` بيشتغل، والـ seccomp بيفضل قافل الـ syscalls الخطيرة.
- **ج. دايمًا الـ 5 capabilities:** أسهل، بس بيكبّر السطح على كل الناس.
- **التوصية:** (ب)، ومعاه escape suite كامل بالإعداد شغّال قبل الدمج، وADR بيعدّل ADR-0043.

### 4. تخزين الـ artifacts والـ cache
- **أ. القرص بس** (`DHOW_BLOB_DIR`) ومعاه interface `Core/Blob`: كفاية لـ VPS واحد (اللي هو المستخدم المستهدف)، ومن غير اعتماديات. بس مافيش أكتر من replica، والـ backup لازم ياخد المجلد.
- **ب. القرص وS3-compatible** (AWS وR2 وMinIO): جاهز للتوسّع والـ artifacts الكبيرة، بس توقيع SigV4 بإيدنا أو اعتمادية، واختبارات زيادة.
- **التوصية:** (أ) في أول نسخة، لأن HA اتأجّل مع M8. وS3 بعدها على نفس الـ interface.

### 5. نسخ المزوّدين
- **أ. GitHub.com وGitLab.com وGitLab self-managed:** نفس الـ API لـ GitLab، وكلهم ينفع يتختبروا في الـ CI.
- **ب. ونزوّد GitHub Enterprise Server:** شركات أكتر، بس مالناش بيئة اختبار ليه، وفيه اختلافات في الـ URLs والنسخ.
- **التوصية:** (أ)، وGHES بعد أول نسخة لما يبقى فيه عميل بيئة اختبار.

### 6. Git عام من غير webhook
بعض سيرفرات Git (git عادي على SSH) مابتبعتش webhooks، أو داو ورا firewall مش متاح من بره.
- **أ. webhook وتشغيل يدوي بس:** أبسط وأقل حمل.
- **ب. ونزوّد polling** (`git ls-remote` من discovery job كل 5 دقايق أو أكتر): بيغطي الحالات دي، بس استهلاك runners وتأخير ومنطق زيادة.
- **التوصية:** (أ) في أول نسخة، والـ polling بعدها لو المستخدمين طلبوه. الـ docs بتشرح سكربت `post-receive` بيبعت الصيغة بتاعتنا.

### 7. حسابات اختبار المزوّدين الحقيقيين
شرط الخلاص محتاج مستودعات حقيقية، وده محتاج من Sabry:
- منظمة GitHub للاختبار، وApp اختبار فيها، وحساب تاني للـ fork.
- group على GitLab.com وتوكن.
- الأسرار دي في GitHub environment محمي (`providers`) على `Khaymah/dhow`.
- **التوصية:** منظمة منفصلة عن `Khaymah` مخصّصة للاختبار، عشان أي تسريب مايلمسش الريبو الأساسي، وتوكنات بأقل scopes وتاريخ انتهاء.

### 8. توكن GitLab
project access tokens على GitLab.com متاحة في Premium وUltimate بس، وعلى self-managed متاحة لكل الـ tiers.
- **أ. أي توكن بالـ scopes الصح** (personal أو project أو group): شغّال لكل الناس، بس الـ personal token مربوط بشخص، ولو ساب الشركة الربط بيقع.
- **ب. project أو group tokens بس:** أنضف، بس مستخدمي GitLab.com المجاني مش هيقدروا يستخدموا داو.
- **التوصية:** (أ)، والواجهة بتوصّي بـ project أو group token، وبتعرض تحذير واضح لو التوكن personal.

## قرارات تانية اتاخدت بالتوصية (Sabry يقدر يرجع فيها)
1. **الأدوار:** جدول الأدوار اللي فوق، وDeveloper بيشغّل يدوي وبيعيد بس مابيلغيش (زي `job.cancel` في M3).
2. **الـ `.dhow.yml` مش موجود:** مفيش run ومفيش check، بدل check أحمر على كل مستودع اتربط.
3. **الـ submodules والـ LFS:** مقفولين في الـ checkout المدمج في أول نسخة.
4. **الـ check على GitHub:** check واحد للـ run كله، مش check لكل job، عشان مانغرقش صفحة الـ PR.
5. **الـ DAG في الواجهة:** قايمة مرتّبة بالمراحل في M4، والرسم في M7.
6. **`isolation`:** `container` بس في أول نسخة، و`gvisor` في الـ YAML بيطلّع خطأ واضح لحد ما gVisor يرجع بعد أول نسخة.
