# ADR-0072: الـ webhooks وcredentials المزوّدين وGit عام (M4.4)

- **الحالة:** مقبول. القرارات 11 و12 و15 مفوَّضة في [`M4-Pipelines.md`](../M4-Pipelines.md)، والباقي تفاصيل تنفيذ مفوَّضة ومكتوبة هنا.
- **التاريخ:** 2026-10-30
- **بيكمّل:** [ADR-0070](0070-ports-in-core-and-project-permissions.md) (الـ ports جوه transaction اللي بينادي) و[ADR-0057](0057-egress-proxy-implementation.md) (نطاقات الـ egress). وطريقة ربط Git نفسها في [ADR-0074](0074-git-connection-oauth-for-setup-deploy-keys-for-access.md).

## السياق
- M4.4 أول مرحلة بيدخل فيها كلام من بره على السيرفر من غير جلسة: Gitea وForgejo وأي سكربت بيبعتوا webhooks على `/webhooks/...`.
- السيرفر لازم يتصل ببره كمان (يجيب الـ host key دلوقتي، وAPIs المزوّدين في M4.5 وM4.6)، وده باب SSRF على الشبكة الداخلية والـ metadata.
- الـ M3 كان عنده قايمة النطاقات الممنوعة في الـ runner بس (`Egress.Ranges.go`)، والسيرفر عنده فحص أخف للـ OIDC والبريد (link-local والـ metadata بس).
- مفيش لسه `Pipelines` (M4.8)، يعني الـ `Trigger` ملوش حد يستقبله.

## القرار

### 1. القرار 11: الـ webhook تلميح، والحقيقة من المزوّد
- الـ webhook بيقول «فيه push على الـ ref ده بالـ sha ده»، ومش أكتر. سر متسرّب ممكن يبعت sha قديم على `main`، فمحدش بيثق فيه لوحده.
- **Git عام:** الـ checkout (M4.7) بيتأكد إن الـ sha موجود في تاريخ الـ ref اللي اتعمله fetch، وإلا `CHECKOUT_REF_MOVED`. والـ run واحد لكل (مشروع، حدث، ref، sha) في M4.8.
- **GitHub وGitLab:** التأكد بالـ API إن الـ sha رأس الـ ref (و`superseded` لو اتحرك) في M4.5 وM4.6. بالنسبة لـ GitLab ده مربوط بـ [ADR-0074](0074-git-connection-oauth-for-setup-deploy-keys-for-access.md): من غير توكن دايم، التأكد بيبقى من الـ checkout زي Git عام لحد ما M4.6 يقرر.
- **من غير ما نستنى M4.8:** `Sources` بيسجّل الـ delivery ويسلّم الـ `Trigger` بس، ومابيعملش أي حاجة تانية بالـ sha.

### 2. القرار 12: credentials المزوّدين متشفّرة بـ `Core/Crypto.Sealer`
- مفتاح مشتق من `DHOW_SECRET_KEY` بـ HKDF و**label لكل نوع**: `dhow/git-deploy-key/v1` للـ deploy keys، و`dhow/git-webhook-secret/v1` لأسرار الـ webhooks. ومفتاح الـ GitHub App والتوكنات بييجوا بـ labels بتاعتهم في M4.5 وM4.6.
- الـ AAD (الـ context) فيه النوع و`"Id"` المستودع (`deploy-key:<id>` و`webhook-secret:<id>`)، فقيمة متشفّرة لمستودع مابتتفكّش لو اتنقلت لصف تاني.
- **الـ deploy key:** Ed25519 بيتولّد في السيرفر، والنص الخاص (صيغة OpenSSH) متشفّر في `"DeployKeySealed"` ومابيرجعش في أي response، والنص العام بس في `"DeployKeyPublic"` والـ API.
- **سر الـ webhook:** 32 byte عشوائي (base64url)، بيرجع **مرة واحدة** في رد الربط الأول أو التجديد، وبعدها مابيظهرش تاني ولا في الـ audit ولا في الـ logs (واختبار بيتأكد).
- **تغيير `DHOW_SECRET_KEY`** بيكسرهم زي تسجيل الـ runners: السر اللي مابيتفكّش بيخلّي الـ webhook يترفض `401` وبيكتب error في الـ log بيقول «جدّد السر». M5 بينقلهم للتشفير المغلّف.

### 3. القرار 15: `/webhooks/...` بره `/api/v1`
- **الـ router:** الـ webhooks متركّبة على الـ root في `App` جنب `/runner-api/`، بـ middleware خاصة بيها: RequestID، وClientIP، وLang، وAccessLog (الـ path والـ status بس، من غير headers)، وRecover، وNoStore، ومهلة 15 ثانية، وBodyLimit بـ `DHOW_WEBHOOK_MAX_BYTES` (افتراضيًا 5 MiB). من غير جلسة ولا CSRF ولا توكن API: السر هو المصادقة، وجلسة أو توكن مع الطلب مابيفرقوش (اختبار route-table للـ webhooks).
- **`policy.Webhook(method, path)`:** نوع جديد `ScopeWebhook`، `POST` بس وتحت `/webhooks/` بس ومن غير إجراء ولا مؤسسة ولا توكنات. الـ rule مابيعدّيش على `auth.Guard` أصلًا (ولو حد حطّه هناك بيترفض لأن مفيش إجراء)، وبيتحقق منه gate صغير في `Sources.Routes.go` من الـ route pattern.
- **الـ routes بتتسجّل من الـ rules نفسها:** `WebhookRoutes` بتلف على `WebhookPolicies()` وبتسجّل كل واحد بـ `r.Method(route.Rule.Method, route.Rule.Path, ...)`، فمفيش route من غير rule.
- **الـ hook (`webhook_rules.py`):** `policy.Webhook` مسموح في `Modules/Sources/Sources.Routes.go` و`Core/Policy` والاختبارات بس، وأي route في الملف ده لازم يتسجّل من الـ rule، ولازم يبقى فيه `policy.Webhook` و`policy.NewTable`. ومعاه `test_webhook_hooks.py`.
- **حد الطلبات:** `Core/RateLimit` بالـ Postgres limiter، 1200 في الدقيقة لكل شبكة (IP، أو /64 لـ IPv6) و300 في الدقيقة لكل `HookId`، قبل أي قراية من قاعدة البيانات. والـ `HookId` الغلط بيتحسب كلّه تحت اسم واحد (`unknown`)، فمابيملاش `"RateLimits"`.

### 4. التحقق من الـ webhook (Git عام)
- **الترتيب:** الحجم، وبعده الحد، وبعده البحث عن المستودع بالـ `HookId` (128 bit عشوائي، مش الـ `"Id"`)، وبعده التوقيع على الـ body الخام بـ `hmac.Equal`، وبعدها بس الـ JSON.
- **المستودع مش موجود:** بنحسب HMAC بسر عشوائي برضه، والرد هو نفس `401 WEBHOOK_REJECTED` بالظبط لأي توقيع غلط أو سر مستودع تاني أو body متعدّل أو `t` قديم أو `HookId` مش موجود. ومن غير صف delivery.
- **التوقيعات الأربعة:** `X-Dhow-Signature: t=<unix>,v1=<hex>` على `t + "." + body` بمهلة ±5 دقايق (لحد 4 قيم `v1` عشان التجديد)، و`X-Forgejo-Signature` و`X-Gitea-Signature` (hex)، و`X-Hub-Signature-256` (`sha256=` + hex). بنتحقق من أول header موجود بالترتيب ده بس، وheader متكرر بيترفض.
- **الـ delivery id:** `X-Dhow-Delivery` مع صيغتنا، و`X-Forgejo-Delivery` أو `X-Gitea-Delivery` أو `X-GitHub-Delivery` (Gitea بيبعته) مع التانيين. لازم `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`. لو التوقيع صح والـ id ناقص: `422`، لأن ده مش هيكشف حاجة لحد معهوش السر.
- **الـ payload:** صيغتنا `{"Ref","Sha"}` بـ `DisallowUnknownFields`، أو push بتاع Gitea/Forgejo (`ref` و`after` و`pusher`). الـ ref بـ `internal/gitref` (فروع وtags بس) والـ sha 40 أو 64 hex صغير، وإلا `422`. وأي حدث غير `push` بيتسجّل `ignored` (`event_not_supported`).
- **الـ actor:** اسم الـ pusher بيتنضّف من حروف التحكم والحروف المخفية و255 حرف بالكتير، وبيروح في الـ `Trigger` كبيانات.

### 5. التكرار (replay)
- **الـ dedupe بحاجتين، مش واحدة:** الـ delivery id **أو** sha256 الـ body، لنفس المزوّد ونفس الـ `HookId`، في آخر 72 ساعة. السبب: الـ delivery id في Gitea مش جوه التوقيع، فمين ما شاف طلب ممكن يعيده بـ id جديد. الـ digest بيمسك ده، والإعادة الحقيقية من المزوّد بيبقى ليها نفس الـ body أصلًا. ومعناها كمان إن نفس `{Ref, Sha}` بصيغتنا مرتين في 3 أيام بيتعمل مرة واحدة، وده المطلوب (run واحد لكل sha).
- **الترتيب جوه الـ transaction:** `pg_advisory_xact_lock` على (المزوّد، `HookId`)، وبعده البحث، وبعده الصف، وبعده `Core/Triggers.Deliver` في نفس الـ transaction. لو الـ `Deliver` فشل، الصف بيتعمله rollback، والإعادة بعدها بتعدّي (مش بتتحسب تكرار).
- **الرد:** `202` للجديد حتى لو `ignored`، و`200` بـ `sources.webhook.duplicate` للتكرار من غير أي أثر.
- **الـ payload مابيتخزّنش:** `"WebhookDeliveries"` فيه المزوّد والـ `HookId` والـ delivery id والـ digest والمستودع والحدث والـ ref والـ sha والنتيجة والسبب بس.

### 6. `"WebhookDeliveries"` مقسّم يوميًا
- `PARTITION BY RANGE ("CreatedAt")` زي `"LogChunks"`، بـ `"Id"` و`"CreatedAt"` primary key مركّب (الـ hook بيقبله من غير تعديل). والـ unique مش ممكن على الأب من غير عمود الـ partition، فالـ dedupe بالـ advisory lock والـ index، مش بقيد.
- functions لوحدها (`EnsureWebhookPartitions` و`DropWebhookPartitions` و`WebhookPartitionReady` و`WebhookPartitionDay`) بنفس شكل بتوع الـ logs، وبتستخدم `LogPartitionToday` و`LogPartitionBound` الموجودين، وبـ `search_path` مقفول.
- **الاحتفاظ 30 يوم بـ `DROP`**، و`DropWebhookPartitions` بيرفض أقل من 4 أيام عشان مايمسحش حاجة جوه نافذة الـ dedupe.
- **الصيانة:** `Sources.Maintain` وقت الإقلاع (لو فشل السيرفر مابيقومش) وكل ساعة، بـ `pg_try_advisory_xact_lock` عشان instance واحدة بس تشتغل. وفي نفس المرور بتمسح الـ deploy key القديم اللي عدّى 24 ساعة.

### 7. `Core/Triggers`
- port بـ `Sink.Deliver(ctx, tx *sqlc.Queries, Trigger) (Accepted, error)` بنفس قواعد ADR-0070. الـ `Trigger` فيه المؤسسة والمشروع والمستودع وصف الـ delivery والمزوّد والحدث والـ ref والـ sha والـ PR والـ fork والـ actor.
- **لحد M4.8:** `App` بيركّب `pendingPipelines`، اللي بيكتب سطر info ويرجّع من غير ما يعمل حاجة. فالـ delivery بيتسجّل `triggered` ومفيش run، ودي حالة مؤقتة ماتطلعش في release. والاختبارات بتركّب `FakeTriggers`.
- **`Core/CommitStatus` مادخلش:** اللي بينادي (`Pipelines`) لسه مش موجود، والـ ADR-0070 بيقول الـ port بيدخل مع أول حد بيناديه. الموجود: `Sources.RecordStatus(ctx, tx, DesiredStatus)` اللي التنفيذ هيستخدمه.

### 8. `"CommitStatusReports"` والـ worker
- صف لكل (مستودع، run)، و`"DesiredState"` و`"DesiredAt"` و`"SentState"`. الـ upsert بيصفّر المحاولات، فآخر حالة بس هي اللي بتتبعت.
- **الـ worker كل 5 ثواني:** `UPDATE ... WHERE "Id" IN (SELECT ... FOR UPDATE SKIP LOCKED LIMIT 20)` بيحجز الصفوف دقيقتين، والإرسال بره الـ transaction. الـ backoff من 10 ثواني بيتضاعف لحد ساعة، وبعد 24 ساعة من `"DesiredAt"` الصف بيتعلّم `"AbandonedAt"`.
- **فاضي لحد M4.5:** Git عام بيرجّع `ErrUnsupported` (القرار 14)، فالصف بيتعلّم متبعت من غير شبكة.

### 9. ربط Git عام
- **`PUT .../repository`** بـ `CloneUrl` و`DefaultBranch`:
  - العنوان بـ `internal/gitsource`، والفرع بـ `internal/gitref.CheckName`، وأي غلط `422` على الحقل من غير أي اتصال.
  - لـ SSH: السيرفر بيجيب الـ host key بـ handshake بس (`ssh.NewClientConn` بـ `HostKeyCallback` بيوقف بعد ما ياخد المفتاح، ومن غير مصادقة)، فوق `netguard`، بمهلة 10 ثواني، ومن غير ما يمسك أي lock في قاعدة البيانات.
  - المفتاح بيتحط في `"PendingHostKey"` ومابيتثبّتش إلا بـ `confirm-host-key` بنفس البصمة `SHA256:`. لو اتغيّر بعد التثبيت، الجديد بيفضل pending والقديم شغّال، وعمرنا ما بنقبله لوحدنا. ولو الـ host أو الـ port اتغيّر، المفتاح المثبّت بيتشال.
  - أول ربط `201` ومعاه السر، وأي `PUT` بعدها `200` من غير سر، وبيحافظ على الـ deploy key والـ `HookId`.
- **`https://`:** لمستودع عام بس، من غير deploy key ولا host key (الـ CHECK بيمنعهم).
- **الحالة:** `host_key_unconfirmed` لـ SSH من غير مفتاح مثبّت، والـ webhooks بتتسجّل `ignored` بالسبب ده لحد التأكيد.
- **تجديد المفتاح:** `rotate-deploy-key` بيخلّي القديم في `"PreviousDeployKey*"` 24 ساعة أو لحد `confirm-deploy-key`.
- **`MintCheckoutCredential` لـ Git عام** بيفك المفتاح الخاص ويرجّعه مع سطر `known_hosts` (`[host]:port` والمفتاح المثبّت)، ومابيرجّعش حاجة لو المفتاح مش مثبّت. ملوش حد بيناديه لحد M4.7 وM4.8.
- **الأخطاء الجديدة:** `REPOSITORY_NOT_LINKED` (404، بكود غير `NOT_FOUND` عشان اختبار الـ route-table يفرّق بينه وبين رفض الصلاحية)، و`GIT_HOST_BLOCKED` (422)، و`GIT_HOST_UNREACHABLE` (502)، و`HOST_KEY_NOT_PENDING` و`HOST_KEY_FINGERPRINT_MISMATCH` و`DEPLOY_KEY_NOT_APPLICABLE` (409)، و`WEBHOOK_REJECTED` (401).
- **الـ audit:** `project.repository_linked` و`project.repository_updated` و`project.host_key_confirmed` و`project.deploy_key_rotated` و`project.deploy_key_confirmed` و`project.webhook_secret_rotated`، تحت مجموعة `project` في الواجهة. والبيانات: `Host` و`Transport` و`DefaultBranch` و`Fingerprint` و`Fields` بس.

### 10. `internal/netguard` و`internal/gitref` و`internal/gitsource`
- **`netguard`:** قايمة النطاقات والـ metadata اتنقلت من الـ runner. `Policy.Denial` بالترتيب: metadata (دايمًا مرفوض)، وبعده `Allowed`، وبعده النطاقات الممنوعة. والـ runner بيستخدمها في `Egress.Ranges.go` ومعاها عناوين الجهاز نفسه زي الأول.
  - `Dialer.Resolve` بيرفض لو **أي** عنوان من الـ DNS ممنوع، و`DialContext` بيتصل بالعنوان اللي اتفحص نفسه (ومعاه `Control` بيفحص تاني وقت الاتصال)، فالـ DNS rebinding مايعدّيش.
  - `HTTPClient`: من غير redirects، و`https` بس (و`http` لو `AllowHTTP`)، ومن غير proxy البيئة، ومهلة 10 ثواني، و`ReadLimited` بـ 5 MiB. لسه ملوش حد بيستخدمه غير الاختبارات لحد M4.5 وM4.6.
  - `DHOW_GIT_ALLOW_PRIVATE_CIDRS` من env بس دلوقتي، والقيمة الغلط بتوقف الإقلاع، ومفيش نطاق أوسع من `/8` لـ IPv4 أو `/16` لـ IPv6.
- **`gitref`:** قواعد `git check-ref-format`، والأشكال الأربعة بس، ورفض أي جزء بيبدأ بـ `-`، والحروف المخفية واتجاه الكتابة (U+202E وأخواتها) مرفوضة. والأسماء العربي في الفروع مقبولة.
- **`gitsource`:** `https://` و`ssh://` و`user@host:path` (اللي بيتحوّل لـ `ssh://`)، والـ host بنفس قواعد `jobspec.ParseEgress`، والمسار ASCII من قايمة محددة ومن غير `..` حتى لو متشفّر بـ `%`.
- **من مراجعة M4.3 (`--` قبل المسار):** `gitsource.ListFileArgs(sha, path)` بيطلّع `git ls-tree -z --full-tree <sha> -- <path>`، و`ReadBlobArgs` بيقرا الـ blob بالـ id اللي اتحقق منه. و`FetchArgs` بيحط `--` قبل العنوان والـ ref. ومعاهم اختبار بيشغّل `git` حقيقي على ملف اسمه `-x.yml` ويثبت إن من غير `--` الأمر بيفشل ومعاه بيقرا الملف. اللي هيشغّلهم هو الـ discovery job والـ checkout (M4.7 وM4.8).

### 11. الجداول
- `"GitConnections"` و`"GitSetupFlows"` اتعملوا بالأعمدة اللي في الخطة، و`"GitSetupFlows"."Kind"` فيه `gitlab_oauth` كمان (ADR-0074). مالهمش queries لحد M4.5 وM4.6، وتنضيف `"GitSetupFlows"` بالـ retention بييجي مع أول صف.
- `"Repositories"."ConnectionId"` بيبقى `NULL` لـ Git عام بس (CHECK)، لأن الـ credential بتاعه لكل مستودع مش لكل مؤسسة.
- `"Repositories"` عليه FK مركّب (`"ProjectId"`, `"OrganizationId"`) على `"Projects"`، زي `"JobAttempts"` على `"Runners"`: كده مستودع مايقدرش يبقى في مؤسسة غير مؤسسة مشروعه. وده مختلف عن `"RoleBindings"."ProjectId"` اللي اتعمل قبل الجدول.
- `"CommitStatusReports"` عليه FK مركّب على `"Repositories"`، و`"RunId"` من غير FK لحد ما `"PipelineRuns"` ييجي.

## اللي اتأجّل
- **`DHOW_GIT_ALLOW_PRIVATE_CIDRS` و`DHOW_WEBHOOK_MAX_BYTES` في «إعدادات السيرفر»** (ADR-0063) مع الـ audit لو اتغيّروا من اللوحة: مع واجهة المزوّدين في M4.10. دلوقتي env بس.
- **فحص جاهزية لـ partitions الـ deliveries في `/health/ready`:** محتاج قيمة جديدة في enum الـ API. الإقلاع بيفشل من غيرها، والصيانة كل ساعة بتعمل 3 أيام قدّام.
- **قايمة آخر الـ deliveries في الـ API:** `LastDeliveryAt` على المستودع كفاية لحد الواجهة في M4.10.
- **`GET /organizations/{id}/git-connections` وإجراء `git.connect`:** مع GitHub (M4.5) وGitLab (M4.6)، لأن Git عام مالوش connection.
- **صورة Forgejo في الـ CI:** اختبارات M4.4 بسيرفر SSH جوه الـ process وطلبات موقّعة بإيدنا. Forgejo الحقيقي في أول PR فيه run كامل (M4.8).

## البدائل اللي اترفضت
- **الـ dedupe بالـ delivery id بس:** في Gitea الـ id مش موقّع، فأي حد شاف طلب يقدر يعيده.
- **unique index على `("Provider", "DeliveryId")`:** مش ممكن على جدول مقسّم من غير `"CreatedAt"`، ومع `"CreatedAt"` مابيمنعش تكرار في يوم تاني.
- **`Mount("/webhooks", ...)` بـ sub-router:** ماكانش هيخلّي الـ routes تتسجّل من الـ rules، والـ hook ماكانش هيعرف يتأكد.
- **قبول الـ host key أول مرة لوحده (TOFU من غير تأكيد):** أول اتصال هو اللي ممكن يتعمله MITM، فالبصمة لازم تتشاف وتتأكد.

## العواقب
- السيرفر بقى عنده نفس قايمة الـ runner للعناوين الممنوعة، وأي اتصال جديد ببره لازم يعدّي على `netguard.Dialer`.
- أي مزوّد جديد بيضيف rule بـ `policy.Webhook` وhandler في `Sources.Routes.go` بس، وتحقق التوقيع والتكرار والـ transaction مشتركين.
- `DHOW_SECRET_KEY` بقى بيشفّر deploy keys وأسرار webhooks كمان، والنسخة الاحتياطية من غيره بتخلّي المستودعات محتاجة ربط من جديد.
