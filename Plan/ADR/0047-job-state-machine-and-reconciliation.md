# ADR-0047: حالات الـ job والـ attempt، والإلغاء بالتدريج، والـ reconciliation بقيود في قاعدة البيانات

- **الحالة:** مقبول (قرار مفوَّض في M3)
- **التاريخ:** 2026-10-27
- **بيكمّل:** [ADR-0042](0042-runner-protocol-and-leasing.md) و[ADR-0030](0030-server-modular-monolith.md) و[ADR-0035](0035-identity-core-implementation.md).

## السياق
- شرط خلاص M3: لو الـ runner اتقتل في نص الـ job، الحالة تترتّب صح.
- قواعد `docs/architecture.md`:
  - مفيش إعادة أوتوماتيك بعد أول خطوة بتغيّر في target.
  - الـ job اللي ضاع بيبقى `lost`، ومابنفترضش إنه نجح.
  - الإلغاء حالة الـ runner بيشوفها، وبيقتل الـ sandbox كله.
- Deploy Center v3 كان بيعيد deployments كاملة مش idempotent من الـ queue.
- M3 مافيهوش pipelines (M4)، فلازم طريقة بسيطة يتعمل بيها job عشان نختبر.

## القرار

### الجداول (موديول `Jobs`، كلها بالأعمدة الأساسية الأربعة)
| الجدول | الأعمدة المهمة |
|---|---|
| `"Jobs"` | `"OrganizationId"`، و`"Name"`، و`"Spec"` (jsonb للـ `JobSpec` بعد التحقق)، و`"SpecDigest"`، و`"Labels"` (`TEXT[]`)، و`"Isolation"`، و`"Priority"`، و`"State"`، و`"Result"`، و`"QueuedAt"` و`"StartedAt"` و`"FinishedAt"`، و`"Deadline"`، و`"QueueDeadline"`، و`"CancelRequestedAt"` و`"CancelRequestedBy"`، و`"AttemptCount"`، و`"MaxInfraRetries"` (افتراضيًا 2)، و`"CreatedByUserId"` |
| `"JobAttempts"` | `"JobId"`، و`"OrganizationId"`، و`"Number"`، و`"RunnerId"`، و`"FencingToken"` (unique، من `"FencingTokens"` sequence)، و`"State"`، و`"FailureReason"`، و`"ExitCode"`، و`"LeasedAt"` و`"StartedAt"` و`"FinishedAt"`، و`"MutationStartedAt"`، و`"CompletionDigest"`، و`"LastSeq"`، و`"LogBytes"` |
| `"Leases"` | `"AttemptId"` (unique)، و`"RunnerId"`، و`"FencingToken"`، و`"State"` (`active` أو `released` أو `expired` أو `revoked`)، و`"ExpiresAt"`، و`"LastHeartbeatAt"` |
| `"AttemptSteps"` | `"AttemptId"`، و`"Index"`، و`"Name"`، و`"Mutating"`، و`"State"`، و`"StartedAt"` و`"FinishedAt"`، و`"ExitCode"` |
| `"LogChunks"` | [ADR-0046](0046-job-logs-storage-and-streaming.md) |

- جداول موديول `Runners` (`"Runners"` و`"RunnerRegistrationTokens"` و`"RunnerSessions"`) في [ADR-0041](0041-runner-identity-and-registration.md).
- الموديولين مابيستوردوش بعض. الـ guard بتاع الـ runner plane في `Core/RunnerAuth` بيدّي `Jobs` الـ principal (المؤسسة والـ labels والـ slots ومستويات العزل والمفاتيح العامة)، والـ implementation من `Runners` بيتركّب في `App`. والإلغاء بيوصل `Jobs` بحدث `runner.revoked` على `Core/EventBus`.
- كل الأوقات من `now()` بتاع قاعدة البيانات، عشان الـ instances مايختلفوش على الساعة.

### الحالات
```
Job:      queued ──► leased ──► running ──► succeeded | failed | canceled | timed_out | lost
             ▲          │           │
             └──────────┴───────────┘  (attempt ضاع قبل أي خطوة mutating، والإعادات لسه متاحة)
          queued ──► canceled | failed(QUEUE_TIMEOUT)

Attempt:  leased ──► running ──► succeeded | failed | canceled | timed_out | lost
          leased ──► lost | canceled
```
- **حالات الـ job النهائية** ما بتتغيّرش. والـ re-run من المستخدم بيعمل job جديد بيشاور على القديم (M4).
- **فرض الحالات في قاعدة البيانات:**
  - CHECK على قيم `"State"`.
  - CHECK إن `"FinishedAt"` موجود لو وبس لو الحالة نهائية.
  - partial unique index بيضمن attempt واحد بس غير نهائي لكل job (`UNIQUE ("JobId") WHERE "State" IN ('leased','running') AND NOT "IsDeleted"`).
  - trigger `BEFORE UPDATE OF "State"` على `"Jobs"` و`"JobAttempts"` بيرفض أي انتقال مش في الرسم (`check_violation`). يعني حتى لو كود غلط حاول ينقل `succeeded` لـ `running`، قاعدة البيانات هترفض.
- **مرة واحدة بالظبط (exactly-once):** كل انتقال `UPDATE ... SET "State" = $to WHERE "Id" = $id AND "State" = ANY($from) RETURNING`، ومعاه فحص الـ fencing في نفس الـ transaction. لو مفيش صف رجع، يبقى فيه حد سبق، والعملية بتتعامل كـ no-op أو تعارض، عمرها ما بتتنفذ مرتين. والأحداث على `EventBus` بتطلع بعد الـ commit بس.

### الإعادة
- الـ attempt بيبقى `lost` لو:
  - الـ lease انتهى.
  - الـ runner اتلغى.
  - الـ runner اترستر وبلّغ إن الـ attempt مات.
- **لو `"MutationStartedAt"` فاضي و`"AttemptCount"` أقل من `1 + "MaxInfraRetries"`:** الـ job بيرجع `queued`، والـ attempt الجاي بياخد fencing أكبر. والـ runner اللي اتلغى مابياخدش الـ job تاني.
- **غير كده:** الـ job بيبقى `lost` نهائي بـ `FailureReason` واضح (`LOST_AFTER_MUTATION` أو `RETRIES_EXHAUSTED`)، ومحتاج إعادة من إنسان.
- **فشل كود المستخدم** (exit code غير صفر) عمره ما بيتعاد أوتوماتيك.

### الإلغاء
1. **`POST .../jobs/{jobId}/cancel`:** بيسجّل `"CancelRequestedAt"`. لو الـ job `queued` بيبقى `canceled` على طول. غير كده، الإشارة بتوصل الـ runner في رد `Heartbeat` أو `Append` الجاي.
2. **الـ runner:** `SIGTERM` للـ process group بتاع الخطوة من خلال الـ agent. وبعد مهلة (`cancel_grace`، افتراضيًا 10 ثواني، وبالكتير 60) `SIGKILL` للـ group. وبعد 5 ثواني كمان `podman kill` و`cgroup.kill` على الحاوية كلها. وبعدين الهدم و`Complete(canceled)`.
3. **`BeginStep` بيرفض** بعد طلب الإلغاء، فمفيش خطوة mutating بتبدأ بعده.
4. **لو الـ runner مارجعش:** بعد المهلة زائد مدة الـ lease، الـ reconciler بيقفل الـ attempt `lost` والـ job `canceled`، مش `queued`، لأن الإلغاء كان متطلب.
- **الوقت:** نفس التدريج عند `"Deadline"` (الـ wall-clock). الـ runner بيطبّقه بنفسه، والسيرفر بيتأكد بعد الـ deadline زائد 30 ثانية، والنتيجة `timed_out`.

### الـ reconciler
- شغّال في كل instance كل 5 ثواني، من غير انتخاب قائد. كل دورة بتاخد صفوف بـ `FOR UPDATE SKIP LOCKED` بحد 100، فأكتر من instance بيشتغلوا مع بعض من غير ما يكرروا شغل.
- **الدورة بتعمل:**
  - الـ leases اللي `"ExpiresAt" < now() - 5s`: بتبقى `expired`، والـ attempt `lost`، وبعدين قاعدة الإعادة.
  - الـ jobs اللي عدّت `"Deadline" + 30s`: بتبقى `timed_out`.
  - الـ jobs اللي `queued` وعدّت `"QueueDeadline"` (افتراضيًا 24 ساعة): `failed` بـ `QUEUE_TIMEOUT`، وده حماية لما مايكونش فيه runner مناسب. والـ job الـ `queued` بيبيّن السبب (`NO_MATCHING_RUNNER`) في الـ API.
  - partitions الـ logs ([ADR-0046](0046-job-logs-storage-and-streaming.md)).
- **عند إقلاع السيرفر:** نفس الدورة بتشتغل مرة قبل ما الـ listener يفتح. ومفيش حالة في الذاكرة لازمة للصح.
- **عند إقلاع الـ runner:**
  - بيقرا الـ journal بتاعه (IDs والـ fencing بس).
  - بيشيل أي حاوية بـ label بتاعه.
  - لكل lease في الـ journal بيبعت `Complete(result=lost, reason=RUNNER_RESTARTED)` بالـ fencing القديم. لو الـ lease لسه صالح، السيرفر بيطبّق قاعدة الإعادة فورًا من غير ما يستنى الانتهاء.
  - وبعدين بيبدأ يطلب leases.

### عمل الـ jobs في M3 وسياسات الـ routes
- `POST /api/v1/organizations/{id}/jobs` بياخد `JobSpec` بنسخة M3 (image بـ digest، وخطوات `run`، وenv من غير أسرار، وحدود، وegress، وlabels، ومستوى العزل، وtimeout). ده **أداة تشغيل في M3** لصاحب المؤسسة والمدير بس. وM4 بيقرر يفضل كـ «تشغيل يدوي» ولا يتشال، لما الـ pipelines تيجي.
- **الإجراءات الجديدة في `Core/Policy`:**

| الإجراء | Owner | Admin | Maintainer | Developer | Viewer |
|---|:-:|:-:|:-:|:-:|:-:|
| `runner.read` | ✅ | ✅ | ✅ | ✅ | — |
| `runner.manage` | ✅ | ✅ | — | — | — |
| `job.read` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `job.create` | ✅ | ✅ | — | — | — |
| `job.cancel` | ✅ | ✅ | ✅ | — | — |
| `log.read` | ✅ | ✅ | ✅ | ✅ | — |

- **الـ routes:**

| Route | الإجراء |
|---|---|
| `GET /organizations/{id}/runners` و`GET /organizations/{id}/runners/{runnerId}` | `runner.read` |
| `PATCH /organizations/{id}/runners/{runnerId}` (الاسم والـ labels والتعطيل) | `runner.manage` |
| `POST /organizations/{id}/runners/{runnerId}/revoke` | `runner.manage` |
| `GET` و`POST /organizations/{id}/runner-registration-tokens` | `runner.manage` |
| `POST /organizations/{id}/runner-registration-tokens/{tokenId}/revoke` | `runner.manage` |
| `GET /organizations/{id}/jobs` و`GET /organizations/{id}/jobs/{jobId}` | `job.read` |
| `POST /organizations/{id}/jobs` | `job.create` |
| `POST /organizations/{id}/jobs/{jobId}/cancel` | `job.cancel` |
| `GET /organizations/{id}/jobs/{jobId}/logs` و`.../logs/stream` | `log.read` |

- **الـ runner plane** ليه نطاق جديد في `Core/Policy` (`ScopeRunner`)، بأنواع:
  - `RunnerRegistration` لـ `Register`.
  - `RunnerChallenge` لـ `Challenge`.
  - `RunnerSigned` لـ `Authenticate`.
  - `RunnerSession` لكل الباقي.
  
  وكل procedure في الـ proto ليه صف.
- **اختبار الـ route-table بيتوسّع:**
  - كل procedure بيترفض بـ session cookie.
  - كل route REST بيترفض بـ runner token.
  - كل procedure بيترفض من runner ملغي، ومن runner من مؤسسة تانية على leases مش بتاعته.
- **الـ hook بتاع الـ routes** بيتوسّع يقبل `policy.Runner(...)` وتركيب الـ Connect handler في `Runners.Routes.go` و`Jobs.Routes.go` بس، مع صفوف policy.
- **أحداث الـ audit:**
  - `runner.registered` و`runner.updated` و`runner.revoked` و`runner.disabled_clone_suspected` و`runner.keys_rotated`.
  - `runner.registration_token_created` و`runner.registration_token_revoked`.
  - `job.created` و`job.cancel_requested`.
  
  تغيّرات حالة الـ job نفسها مش audit، دي في الـ job.

## البدائل اللي اتدرست
- **الحالة على الـ job بس من غير attempts:** بتضيّع تاريخ المحاولات، ومافيش مكان للـ fencing لكل محاولة.
- **الـ lease جوه `"JobAttempts"`:** جدول أقل، لكن الـ heartbeat كل 10 ثواني كان هينفخ أهم جدول.
- **انتخاب قائد للـ reconciler بـ advisory lock:** نقطة فشل زيادة. الـ `SKIP LOCKED` كفاية وأبسط.
- **فرض الحالات في الكود بس:** أي bug أو query جديد ممكن يكسرها، والـ trigger بيمسكها.
- **إعادة أوتوماتيك لأي attempt ضاع:** بيرجّع مشكلة v3 بالظبط.

## العواقب
- كل كود بينقل حالة لازم يعدّي على دالة الانتقال الواحدة، والـ trigger هو شبكة الأمان.
- **اختبارات سباق:**
  - 100 runner على job واحد: lease واحد بس.
  - `Complete` و`Heartbeat` والـ reconciler في نفس اللحظة.
  - إلغاء أثناء `BeginStep`.
- **أرقام الـ migrations** بتاخد الرقم الفاضي الجاي وقت الـ merge، لأن M2.2 لحد M2.5 ممكن يضيفوا migrations قبلها.
- `job.create` في M3 بيدّي صاحب المؤسسة والمدير إنهم يشغّلوا أي image على أي runner في المؤسسة. ده نفس اللي الـ pipelines هتعمله، بس من غير مراجعة كود، فهو مقصور عليهم.
