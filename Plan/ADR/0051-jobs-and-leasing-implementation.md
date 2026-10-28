# ADR-0051: تنفيذ الـ jobs والـ leases في M3.3: الـ triggers، والـ claim، والرسائل الموقّعة، وقواعد الإنهاء، والـ reconciler

- **الحالة:** مقبول (قرار مفوَّض في M3.3)
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0042](0042-runner-protocol-and-leasing.md) و[ADR-0045](0045-secrets-transport-and-masking.md) و[ADR-0047](0047-job-state-machine-and-reconciliation.md) و[ADR-0050](0050-runner-identity-implementation.md).

## السياق
ADR-0042 حدّد دورة الـ lease والـ fencing، وADR-0045 حدّد نقل الأسرار، وADR-0047 حدّد الجداول والحالات وقواعد الإعادة والـ reconciler. التنفيذ طلّع أسئلة لازم تتقفل عشان M3.4 (الـ logs) وM3.5 (الـ runner) يبنوا عليها: الرسم بيتفرض في قاعدة البيانات إزاي بالظبط، والـ claim بيمنع إن runner ملغي ياخد job إزاي، والـ digest بتاع `Complete` بيتحسب من إيه، وإثبات الامتلاك في `Start` بيتوقّع على إيه، وأنهي فشل بيتعاد.

## القرار

### قاعدة البيانات
- **الـ migration** رقمها `00031_jobs.sql`، لأن M2.3 بياخد من 00007، والـ goose بيقبل الفراغ.
- **`"Result"` اتشال من `"Jobs"`:** الحالة النهائية هي النتيجة نفسها، والسبب في `"FailureReason"` (نص) على الـ job والـ attempt (`QUEUE_TIMEOUT` و`LOST_AFTER_MUTATION` و`RETRIES_EXHAUSTED` و`CANCELED` وأسباب الـ proto زي `STEP_FAILED`).
- **أعمدة زيادة عن ADR-0047:**
  - `"Leases"."GrantDigest"` (SHA-256 لـ `grant_bytes`) عشان إثبات الامتلاك في `Start`، و`"EndedAt"`، و`"JobId"` و`"OrganizationId"` منسوخين عشان الـ reconciler مايعملش join زيادة.
  - `"AttemptSteps"."FailureReason"`، وحالات الخطوة `running` و`succeeded` و`failed` و`interrupted` (الخطوة اللي كانت شغّالة لما الـ attempt خلص).
- **الانتقالات بتتفرض بـ 4 triggers** (`"Jobs"` و`"JobAttempts"` و`"Leases"` و`"AttemptSteps"`) من نوع `BEFORE INSERT OR UPDATE OF "State"`:
  - الإدخال لازم بالحالة الأولى (`queued` و`leased` و`active` و`running`).
  - أي انتقال مش في الرسم بيرجع `check_violation` (`23514`) برسالة `cannot move from X to Y`. نفس الحالة لنفسها مسموحة (no-op).
  - الرسم مكتوب في الكود كمان (`Modules/Jobs/Models`)، والـ finalize بيسأل الكود قبل ما يكتب. واختبار بيجرّب كل زوج من الحالات على الكود وعلى الـ trigger مباشرة بـ SQL، فأي فرق بينهم بيفشّل الـ CI.
- **إضافة على رسم ADR-0047:** `leased → failed` للـ attempt والـ job. ده الفشل اللي بيحصل قبل `Start`، زي `IMAGE_PULL_FAILED` أو سر ناقص، وكان من غيره لازم يتسجّل `lost` وهو مش ضياع.
- **الصفوف النهائية مقفولة:** الـ trigger نفسه بيرفض أي تعديل على job أو attempt أو lease أو خطوة خلصت، ما عدا `"IsDeleted"` و`"UpdatedAt"` (الحذف soft). المقارنة بـ `to_jsonb(NEW)` و`to_jsonb(OLD)` من غير العمودين دول، فأي عمود جديد بيتغطّى لوحده.
- **المفاتيح مربوطة بالمؤسسة:** `"JobAttempts"` بيشاور على (`"Jobs"."Id"`, `"OrganizationId"`) وعلى (`"Runners"."Id"`, `"OrganizationId"`)، و`"Leases"` بيشاور على (`"Id"`, `"JobId"`, `"OrganizationId"`, `"RunnerId"`, `"FencingToken"`) في `"JobAttempts"`. يعني قاعدة البيانات نفسها بترفض attempt على runner من مؤسسة تانية، أو lease بـ fencing غير بتاع الـ attempt. والـ unique index على `"Runners"` (`"Id"`, `"OrganizationId"`) اتعمل في نفس الـ migration لأنه شرط للـ FK.
- **`"Jobs"."Image"`** عمود لوحده، فقايمة الـ jobs مابتقراش الـ spec ولا بتفكّه.
- **القيود:** `"FinishedAt"` موجود لو وبس لو الحالة نهائية، و`"Deadline"` فاضي في `queued`، و`"AttemptCount" <= "MaxInfraRetries" + 1`، والـ attempt الـ `running` ليه `"StartedAt"`، ومفيش mutation قبل البداية، وطلب الإلغاء معاه مين طلبه، وattempt مفتوح واحد لكل job، وfencing unique في `"JobAttempts"` و`"Leases"`.
- **الوقت:** كله من `now()` بتاع قاعدة البيانات. `"Deadline"` بيتحط في `Start` (`now()` + `timeout_seconds`) وبيتمسح لما الـ job يرجع للطابور. و`"QueueDeadline"` بيتحسب مرة واحدة عند الإنشاء (24 ساعة) ومابيتجددش مع الإعادة.

### الـ claim (`LeaseService.Lease`)
- transaction واحدة:
  1. `pg_advisory_xact_lock(hashtextextended('dhow.runner-slots:' || runner_id, 0))`.
  2. عدّ الـ leases النشطة اللي `"ExpiresAt" > now()` (الـ lease المنتهي اللي الـ reconciler لسه ماشافهوش مابياخدش slot، لأن الـ runner قفل نفسه)، وقراية `"Runners"."State"`.
  3. job واحد `FOR UPDATE SKIP LOCKED` بالأولوية وبعدين `"QueuedAt"`، في مؤسسة الـ runner، والـ labels بتاعته `<@` labels الـ runner (الأدمن والقدرات)، والعزل عنده، ومن غير ما يكون نفس الـ runner فشل عليه بـ `RESOURCE_REQUEST_EXCEEDS_RUNNER`.
  4. الـ fencing من `nextval`، والـ IDs بتتولد uuidv7 في Go، عشان الـ grant يتبني ويتوقّع ويتحسب الـ digest بتاعه قبل الإدخال. و`issued_at` هو `now()` بتاع الـ transaction.
- **`Jobs` بيقرا `"Runners"` (قراية بس)** في الـ claim والـ reconciler و`QueuedReason`. ده استثناء مقصود من «كل موديول بجداوله»: الـ principal بيتقرا في الـ gate، وبين الـ gate والـ claim ممكن الأدمن يلغي الـ runner. القراية في نفس الـ transaction بعد القفل، والـ handler بتاع `runner.revoked` بياخد نفس القفل، فأي ترتيب للاتنين بيطلع صح. ومفيش import بين الموديولين.
- **الـ long-poll:**
  - الاشتراك قبل أول محاولة، فمفيش إشعار بيضيع بين المحاولة والاستنى.
  - hub واحد لكل instance بـ `LISTEN dhow_jobs_queued` على اتصال لوحده (مش من الـ pool)، والـ payload هو المؤسسة، والـ hub بيصحّي المشتركين بتوعها بس. وبعد أي اتصال جديد بيصحّي الكل لأن الإشعارات اللي فاتت ضاعت. وفيه poll احتياطي كل ثانيتين.
  - `NOTIFY` بيطلع في نفس transaction الإنشاء أو الرجوع للطابور، فبيوصل بعد الـ commit بس.
  - `free_slots = 0` بيرجع على طول مهما كان `wait_seconds`، ومن غير ما يلمس قاعدة البيانات.
  - **كل نداء فيه claim** (فوري أو long-poll) بيعدّي على token bucket لكل runner في الذاكرة: 5 نداءات burst، وواحد كل ثانية بعدها، والزيادة `RESOURCE_EXHAUSTED` بـ `LEASE_RATE_LIMITED` و`Retry-After: 1`. فـ `wait_seconds = 0` مابقاش طريق يعدّي الحدود. الـ buckets بتتنضّف لما تعدّي 10000 runner.
  - الـ long-poll كمان: poll واحد مفتوح لكل runner (`POLL_IN_PROGRESS`)، و`DHOW_RUNNER_MAX_POLLS` لكل instance (`TOO_MANY_POLLS`)، والاتنين معاهم `Retry-After: 5`.
  - **`DHOW_RUNNER_MAX_POLLS` افتراضيًا 500** (25 لكل اتصال من الـ 20 في الـ pool)، والحد الأقصى 10000. الـ poll وهو مستني مابيمسكش اتصال، لكن كل صحيان بيعمل transaction claim قصيرة، فالرقم مربوط بالـ pool مش بعدد الـ runners.
  - **الـ poll الاحتياطي** كل 5 ثواني بـ jitter (بين 2.5 و7.5)، فالـ polls اللي فتحت مع بعض مابتصحاش مع بعض. الـ LISTEN هو الطريق الأساسي، والاحتياطي للإشعارات اللي ضاعت بس.
  - الـ runner المعطّل بيستنى المدة كلها من غير claim.
  - عند إيقاف السيرفر (`RegisterOnShutdown`) كل الـ polls المفتوحة بترجع فاضية على طول، عشان الإيقاف مايستناش 25 ثانية.

### الرسائل الموقّعة الجديدة (`internal/runnersig`)
بنفس صيغة ADR-0050 (label، وكل حقل بطوله)، والأرقام 64 bit من غير طول:
- **الـ grant:** `dhow/lease-grant/v1 ‖ grant_bytes`، ومتوقّع بمفتاح الـ runner plane.
- **إثبات الامتلاك في `Start`:** `dhow/lease-start/v1 ‖ lease_id ‖ fencing_token ‖ SHA-256(grant_bytes)` بمفتاح الـ runner. السيرفر بيقارنه بـ `"GrantDigest"` اللي اتخزّن، فتوكن مسروق مايجيبش أسرار.
- **digest الإنهاء:** `SHA-256(dhow/attempt-completion/v1 ‖ attempt_id ‖ fencing_token ‖ result ‖ exit_code ‖ failure_reason)`. السيرفر بيحسبه ويقارنه بالمبعوت، وأي فرق `INVALID_ARGUMENT`. يعني «نفس الـ digest» معناه نفس النتيجة بالظبط لنفس الـ attempt، ومفيش runner يقدر يعيد digest attempt قديم على attempt جديد.
- **الـ AAD بتاع الأسرار:** `dhow/secrets/v1 ‖ lease_id ‖ fencing_token ‖ attempt_id ‖ runner_id`، والـ `info` هو نفس الـ label.
- **`SecretBundle`** (أسماء وقيم) اتضاف في `job.proto` كإضافة مش كسر، وهو الـ plaintext اللي بيتشفّر.

- **حد الطابور:** 1000 job في `queued` لكل مؤسسة، والزيادة `429 JOB_QUEUE_FULL` وقت الإنشاء. العدّ تحت advisory lock على طابور المؤسسة في نفس transaction الإدخال، فطلبات متوازية ماتعدّيش الحد.
- **مفيش GIN index على `"Labels"`:** الـ claim بيمشي على `"IX_Jobs_Queue"` بالترتيب (المؤسسة والحالة والأولوية والوقت) و`LIMIT 1`، وبيفلتر الـ labels على الصفوف اللي بيقابلها. GIN مابيدّيش ترتيب، فالـ planner كان هيجيب كل الـ jobs المطابقة ويرتّبهم. ولو مؤسسة عندها jobs كتير بـ labels محدش عنده، الحد بتاع الطابور (1000) هو اللي بيحدّ الشغل.

### الـ RPCs
- **كل RPC بيلمس lease** بيبدأ بـ `SELECT ... FOR UPDATE OF l` على `"Id"` والـ fencing و`"RunnerId"` (الـ principal) و`active` و`"ExpiresAt" > now()`. أي فشل (مش موجود، أو بتاع runner تاني أو مؤسسة تانية، أو منتهي، أو fencing قديم) بيرجّع نفس الرد: `FAILED_PRECONDITION` بـ `LEASE_LOST`.
- **ترتيب الأقفال** في كل المسارات: lease وبعدين attempt وبعدين job. والإلغاء من الـ API بيقفل الـ job بس، والـ claim بيقفل job في `queued` بس. فمفيش دورة.
- **`Start`:**
  - الـ attempt لازم `leased`، وإلا `ATTEMPT_ALREADY_STARTED`. والتوقيع غلط أو ناقص: `INVALID_RUNNER_CREDENTIALS`. وطلب إلغاء قبل البداية (`FOR SHARE` على الـ job): `CANCEL_REQUESTED`، والأسرار مابتطلعش.
  - الأسرار بتتجاب وتتشفّر جوه الـ transaction قبل الـ commit، فلو الـ commit فشل الـ bundle بيتشال، ولو التشفير فشل الـ attempt مابيبقاش `running`.
  - سر مش موجود: الـ attempt والـ job `failed` بـ `SECRETS_UNAVAILABLE` من غير إعادة (الإعادة مش هتجيبه)، والرد `FAILED_PRECONDITION` بـ `SECRETS_UNAVAILABLE`. والـ log فيه أسماء الأسرار بس، تحت المفتاح `references` (الـ logger بيخفي أي مفتاح فيه `secret`).
  - `Start` بيمدّ الـ lease ويحط `"Deadline"`.
- **`Heartbeat`** شغّال في `leased` كمان، عشان الـ runner بيسحب الـ image قبل `Start`.
- **`BeginStep`:** `running` بس (`ATTEMPT_NOT_RUNNING`)، والإلغاء بيتقرا بـ `FOR SHARE` فمايتسابقش مع طلب الإلغاء، و`mutating` لازم يطابق الـ spec، والإدخال `ON CONFLICT DO NOTHING` فالإعادة آمنة. و`"MutationStartedAt"` بيتكتب قبل الرد.
- **`EndStep`:** `STEP_NOT_STARTED` لو مفيش `BeginStep`، والإعادة على خطوة خلصت OK.
- **`Complete`:**
  - الـ digest لازم 32 byte ومطابق للحساب.
  - attempt خلص بنفس الـ digest: OK. بـ digest تاني: `ALREADY_EXISTS` بـ `COMPLETION_CONFLICT`. خلص من غير digest (الـ reconciler قفله): `LEASE_LOST`.
  - الـ lease انتهى والـ attempt لسه مفتوح: `LEASE_LOST`، وسطر في log السيرفر فيه الـ attempt والنتيجة (سطر الـ `system` في `"LogChunks"` بييجي مع M3.4).
  - `succeeded` من `leased` مرفوض (`ATTEMPT_NOT_RUNNING`)، و`succeeded` بـ exit code `INVALID_ARGUMENT`.
  - **السبب لازم يكون من أسباب النتيجة دي** وإلا `INVALID_ARGUMENT` بـ `VALIDATION_FAILED: FailureReason`: `succeeded` من غير سبب، و`failed` بأسباب الـ job أو البنية التحتية (`STEP_FAILED` و`OOM_KILLED` و`PIDS_LIMIT_REACHED` و`IMAGE_PULL_FAILED` و`SHELL_NOT_FOUND` و`SPEC_REJECTED` و`RESOURCE_REQUEST_EXCEEDS_RUNNER` و`SANDBOX_FAILED` و`RUNNER_DISK_FULL`)، و`canceled` بـ `CANCELED`، و`timed_out` بـ `TIMED_OUT`، و`lost` بـ `RUNNER_RESTARTED` أو `LEASE_LOST`. والسبب الفاضي بياخد الافتراضي (`STEP_FAILED` و`CANCELED` و`TIMED_OUT` و`LEASE_LOST`). وأي رقم مش معرّف في الـ enum مرفوض.
  - و`EndStep` بيقبل أسباب الخطوة بس (`STEP_FAILED` و`OOM_KILLED` و`PIDS_LIMIT_REACHED` و`TIMED_OUT` و`CANCELED` و`SHELL_NOT_FOUND`).

### قواعد الإنهاء والإعادة (`finalize` واحدة لكل المسارات)
| الـ attempt | الـ job |
|---|---|
| `succeeded` | `succeeded` |
| `canceled` | `canceled` |
| `timed_out` | `timed_out` |
| `failed` بسبب الـ job نفسه (`STEP_FAILED` و`OOM_KILLED` و`PIDS_LIMIT_REACHED` و`SHELL_NOT_FOUND` و`SPEC_REJECTED` و`TIMED_OUT` و`SECRETS_UNAVAILABLE`) | `failed` بنفس السبب، من غير إعادة |
| `lost`، أو `failed` بنية تحتية (`IMAGE_PULL_FAILED` و`SANDBOX_FAILED` و`RUNNER_DISK_FULL` و`RESOURCE_REQUEST_EXCEEDS_RUNNER`) | الإلغاء متطلب: `canceled`. الـ mutation بدأ: `lost` بـ `LOST_AFTER_MUTATION` (أو `failed` بالسبب لو فشل بنية). `"AttemptCount" < 1 + "MaxInfraRetries"`: `queued` ومعاه `NOTIFY`. غير كده: `lost` أو `failed` بـ `RETRIES_EXHAUSTED` |

- الـ lease بيبقى `released` (`Complete`) أو `expired` (انتهى) أو `revoked` (الـ runner اتلغى أو الـ deadline عدّى)، والخطوات الشغّالة بتبقى `interrupted`.
- `Complete(lost, RUNNER_RESTARTED)` والـ lease لسه صالح بيطبّق نفس القاعدة على طول من غير ما يستنى الانتهاء.

### الـ reconciler
- 6 مسارات، كل صف في transaction لوحده (`LIMIT 1 FOR UPDATE SKIP LOCKED`) وبحد 100 لكل مسار في الدورة:
  - leases عدّت `"ExpiresAt"` بـ 5 ثواني: `expired` و`LEASE_LOST`.
  - leases لـ runner حالته `revoked`: `revoked` و`RUNNER_REVOKED`.
  - **attempt في `leased` اتطلب إلغاؤه من أكتر من دقيقة:** الـ attempt والـ job `canceled` والـ lease `revoked`. الـ runner بياخد دقيقة يبلّغ `Complete(canceled)` بنفسه، وبعدها السيرفر بيقفله حتى لو الـ runner لسه بيبعت heartbeat.
  - **attempt في `leased` من أكتر من 10 دقايق (مهلة البداية):** `lost` بـ `START_TIMEOUT` والـ lease `revoked`، وبعدين قاعدة الإعادة. من غيرها runner بيبعت `Heartbeat` من غير `Start` كان هيمسك الـ job للأبد. والـ 10 دقايق كفاية لسحب image كبيرة.
  - jobs عدّت `"Deadline"` بـ 30 ثانية: الـ attempt والـ job `timed_out` والـ lease `revoked`.
  - jobs في `queued` عدّت `"QueueDeadline"`: `failed` بـ `QUEUE_TIMEOUT`.
- كل 5 ثواني في كل instance، ومرة عند الإقلاع قبل الـ listener. ولو المرة دي فشلت، السيرفر مابيقومش (fail closed).
- **`runner.revoked`:** handler في `Jobs` بياخد قفل الـ slots بتاع الـ runner ويقفل كل leases بتاعته (`FOR UPDATE`، فبيستنى أي RPC شغّال عليها). لو فشل، بيسجّل الخطأ ويرجّع `nil`، عشان إلغاء الـ runner اللي اتعمله commit مايرجعش 500 للأدمن، والـ reconciler بيلقط الباقي في 5 ثواني.

### الأسرار
- `SecretResolver` فيه `Accepts` (وقت الإنشاء، لو الـ spec فيه refs) و`Resolve` (في `Start` بس، بالمؤسسة والـ job والـ attempt والأسماء). البيئة والـ trigger بيتضافوا مع M4 وM5.
- الإنتاج بيركّب `NoSecrets`، فأي job فيه `SecretRef` بيترفض وقت الإنشاء بـ `422 SECRETS_UNAVAILABLE`. والاختبارات بتركّب `FakeSecrets` من `Infra.Secrets`.
- القيم بتتمسح من الذاكرة بعد التشفير (best effort)، ومش في قاعدة البيانات ولا الـ grant ولا أي رد ولا log، وفيه اختبار بيدوّر عليها بكل الصيغ.

### الـ REST API
- 4 routes بالإجراءات اللي في ADR-0047 (`job.read` و`job.create` و`job.cancel`)، ومعاهم صف في الـ route-table لكل دور.
- الإلغاء idempotent: `queued` بيبقى `canceled` على طول، و`leased` أو `running` بيتعلّم، وjob `canceled` بيرجّع `200`، وjob خلص بحالة تانية `409 JOB_FINISHED`. والحدث `job.cancel_requested` بيطلع أول مرة بس.
- `QueuedReason: NO_MATCHING_RUNNER` لو مفيش runner نشط في المؤسسة عنده الـ labels والعزل.
- الـ spec بيتطبّع (القيم الافتراضية، والـ labels والـ egress مترتبين من غير تكرار) قبل التخزين، وبيتخزّن protojson بأسماء الـ proto، والـ digest (`SpecDigest`) على الـ deterministic marshal بعد التطبيع.
- `BodyLimit` لـ `POST /jobs` نص ميجا. و`jobspec.MaxSpecBytes` محسوب: `MaxGrantBytes` (256 KiB، نفس `RunnerReadMaxBytes`، واختبار بيتأكد إنهم متساويين) ناقص 4 KiB للـ IDs والتوقيع والتغليف (الفعلي أقل من 400 byte). واختبار بيبني spec بالحجم الأقصى بالظبط ويتأكد إن `LeaseResponse` كله أقل من الحد، وإن byte زيادة بيترفض.

### حدود `internal/jobspec`
| الحقل | الحد |
|---|---|
| `Image` | بالـ digest إجباري (`name@sha256:<64 hex>`)، و512 حرف |
| `Steps` | من 1 لـ 100، والاسم 100 حرف من غير حروف تحكم، والـ script لحد 64 KiB من غير NUL، والـ timeout من 0 لحد timeout الـ job |
| `Env` | 100 متغير في كل مستوى، والاسم `[A-Za-z_][A-Za-z0-9_]*` لحد 128، والقيمة لحد 32 KiB، والـ prefix `DHOW_` محجوز |
| `Secrets` | 50، و`AsEnv` أو `AsFile` بالظبط، والملف اسم بس (من غير `/` ولا `..`)، ومفيش هدفين بنفس الاسم، والـ env مايغطيش env عادي |
| `Resources` | CPU من 100 لـ 64000 millis (افتراضي 1000)، والذاكرة من 64 MiB لـ 64 GiB (2 GiB)، والـ PIDs من 32 لـ 32768 (1024)، والقرص من 64 MiB لـ 100 GiB (4 GiB) |
| `TimeoutSeconds` | من 1 لـ 86400 (افتراضي 3600) |
| `Network.Egress` | 64، بصيغة `host` أو `*.host` أو `host:port`، ومن غير IP literals، زي `docs/pipeline-spec.md` (التحقق الكامل مع M3.7) |
| `Labels` | 32، بصيغة labels الـ runner، والـ prefix `dhow.` مسموح هنا لأنه بيطلب قدرة |

## قيود معروفة
- **إثبات الامتلاك في `Start` بس:** باقي الـ RPCs (`Heartbeat` و`BeginStep` و`EndStep` و`Complete`) بالتوكن والـ fencing. توكن مسروق ومعاه lease ref يقدر يمدّ الـ lease أو ينهيه بنتيجة غلط، لكن مايقدرش ياخد أسرار. ممكن نضيف توقيع على `Complete` بعدين، والـ digest جاهز يتوقّع.
- **`Resolve` جوه الـ transaction:** الـ resolver بيتنادى والـ lease مقفول. ده كويس مع `NoSecrets` والاختبار، لكن مخزن أسرار خارجي (M4 وM5) لازم يتنادى بـ timeout قصير (أقل من مدة الـ lease) عشان مايمسكش القفل والاتصال.
- **الـ audit بعد الـ commit:** `job.created` و`job.cancel_requested` بيطلعوا بعد ما الـ transaction تخلص، زي باقي الموديولات. لو السيرفر وقع بينهم، الـ job موجود من غير حدث audit. الحل الكامل (outbox) قرار عام مش خاص بالـ jobs.
- **الـ spec بيشوفه كل اللي عنده `job.read`، والـ Viewer منهم:** الـ env والـ scripts ظاهرين في الـ API. أي credential لازم يبقى `SecretRef` مش env، والتحذير ده في وصف `Env` في `openapi.yaml` وفي `docs/pipeline-spec.md`.

## البدائل اللي اتدرست
- **جدول للانتقالات بدل `IN` في الـ trigger:** كان هيحتاج الأعمدة الأساسية وmigration لأي تعديل. والـ `IN` المكتوب بإيدينا أوضح، والاختبار بيقارنه بالكود في كل زوج.
- **digest من الـ runner من غير تحقق:** كان هيخلّي «نفس الـ digest» ممكن لنتيجتين مختلفين، ويخلّي digest قديم يتقبل على attempt تاني.
- **IDs الـ lease من قاعدة البيانات:** كان هيحتاج `UPDATE` بعد الإدخال عشان الـ digest، والـ grant كان هيتبني على مرحلتين.
- **حالة الـ runner من الـ principal بس:** بين الـ gate والـ claim ممكن الإلغاء يحصل، والـ lease كان هيتعمل لـ runner ملغي.
- **handler الإلغاء يرجّع الخطأ:** الأدمن كان هياخد 500 على إلغاء اتعمله commit، والإعادة مش هتطلّع الحدث تاني.
- **إعادة كل فشل قبل الـ mutation:** فشل كود المستخدم (`STEP_FAILED` و`OOM_KILLED`...) عمره ما بيتعاد (ADR-0047)، وسر ناقص مش هييجي بالإعادة.
- **GIN index على `"Labels"`:** اتشرح فوق، الـ claim محتاج الترتيب أكتر من الفلترة.
- **فاصل ثابت بين النداءات الفورية بدل token bucket:** كان هيرفض runner بيعمل `Lease` وراه `Lease` على طول بعد ما خلّص job، والـ burst الصغير بيسمح بده.
- **LISTEN على اتصال من الـ pool:** بيحجز اتصال من الـ 20 للأبد، وبيقع مع أي إعادة تدوير للـ pool.

## العواقب
- **كل job جديد بيصحّي كل الـ polls المفتوحة في مؤسسته،** وواحد بس بيكسب بـ `SKIP LOCKED`. ده مقبول لحد مئات الـ runners في المؤسسة، ولو كبر بنوزّع الإشعار بالـ labels بعدين.
- `Jobs` بيقرا `"Runners"` من غير import، ولو الجدول اتغيّر لازم الـ queries دي تتراجع.
- **اتأجّل:**
  - رفض `RESOURCE_REQUEST_EXCEEDS_RUNNER` وقت الإنشاء: الـ runner مابيبلّغش عن سقف موارده في `Register` (M3.2)، فده محتاج حقل في الـ proto مع M3.5. لحد كده الـ runner بيرفض في `Start` أو قبله، والـ job بيتعاد على runner تاني.
  - سطر `system` في الـ log عند `Complete` بعد انتهاء الـ lease: مع M3.4.
  - الـ token bucket لكل runner ورد «الـ pool مشغول» (ADR-0042): مش في M3.3.
  - `DHOW_FENCING_TOKEN` في env الـ job: الـ runner بيحطه في M3.5، والـ prefix محجوز من دلوقتي.
