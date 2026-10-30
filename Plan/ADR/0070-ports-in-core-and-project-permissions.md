# ADR-0070: الـ ports في `Core` والصلاحيات على المشروع (M4.3)

- **الحالة:** مقبول. القرار 1 مفوَّض في [`M4-Pipelines.md`](../M4-Pipelines.md)، والباقي تفاصيل تنفيذ مفوَّضة ومكتوبة هنا.
- **التاريخ:** 2026-10-30
- **بيكمّل:** [ADR-0030](0030-server-modular-monolith.md) (الموديول مابيستوردش موديول تاني) و[ADR-0047](0047-job-state-machine-and-reconciliation.md) (حالات الـ job). وبيستخدم `"RoleBindings"` بتاع M2.3.

## السياق
- M4 فيه أربع موديولات جديدة (`Projects` و`Sources` و`Pipelines` و`Artifacts`) لازم يكلّموا بعض ويكلّموا `Jobs`، والموديول ممنوع يستورد موديول تاني.
- الطريق الوحيد لحد دلوقتي هو `Core/EventBus`، وده بينشر **بعد** الـ commit ومن غير outbox. يعني لو الـ process وقع بين الـ commit والنشر، الحدث بيضيع. ده مقبول للـ audit (ومتابَع في `Status.md` بسبب `runner.revoked`)، بس مش مقبول لـ «اعمل الـ jobs بتاعة الـ run ده»: حدث ضايع هنا معناه run واقف للأبد.
- فيه نمط موجود فعلًا: `Core/RunnerAuth` بيعرّف `Verifier` كـ interface، و`Runners` بينفّذه، و`App` بيركّبه. ونفس الفكرة في `auth.TokenResolver` اللي `Tokens` بينفّذه.
- `"RoleBindings"` فيه `Scope = 'project'` و`"ProjectId"` من M2، ومستني جدول المشاريع.

## القرار

### 1. الـ ports في `Core` بـ interfaces بس
- كل port package في `Core/<Port>` فيه **interfaces وأنواع بيانات وثوابت بس**: مفيش `func` ولا `var` على مستوى الـ package.
- الـ port بيتنفّذ في الموديول اللي بيملك الجداول، و`App` هو اللي بيركّب التنفيذ في الموديول اللي بينادي. فالموديولين مابيعرفوش بعض، وكل واحد بيعرف الـ port بس.
- الأسماء المتخطط لها: `JobQueue` (تنفيذه في `Jobs`، في M4.3)، و`Triggers` و`CommitStatus` (M4.4)، و`RunEvents` (M4.8)، و`ArtifactIndex` (M4.9).
- **كل port بيدخل الكود مع أول حد بيناديه**، مش قبلها. M4.3 فيه `JobQueue` بس، لأن الـ interface من غير حد بينادي هو تخمين للتوقيع، والمرحلة اللي بتستخدمه هي اللي بتعرف شكله. أما قواعد الـ hook فبتعرف الأسماء الخمسة من دلوقتي، فأي port منهم بيتقفل من أول سطر.

### 2. الـ port بيشتغل جوه transaction اللي بينادي
- كل method في الـ port بتاخد `*sqlc.Queries` مربوط بالـ transaction بتاع اللي بينادي، ومابتفتحش transaction لوحدها. الـ `sqlc` package واحد لكل الموديولات، فده مابيكسرش اتجاه الاستيراد (`Core` مسموحله يستورد `Database`).
- النتيجة: يا كله يتسجّل يا ولا حاجة. الـ run والـ jobs بتوعه، أو الـ delivery والـ run، بيتعملوا commit مع بعض أو rollback مع بعض.
- التنفيذ **مابينشرش على الـ EventBus** جوه الـ port، لأن النشر قبل الـ commit ممكن يسجّل حاجة اتعملها rollback. اللي بينادي هو اللي بيسجّل الـ audit بتاعه بعد الـ commit (مثلًا `Pipelines` بيسجّل الـ run، مش كل job).
- `pg_notify` اللي بيصحّي الـ runners بيتبعت جوه نفس الـ transaction، وPostgres بيوصّله بعد الـ commit بس، فالـ rollback مابيصحّيش حد.
- **ليه مش `pgx.Tx`؟** `dbtx.Run` بيدّي `*sqlc.Queries`، والتنفيذ محتاج الـ queries مش الـ connection. و`*sqlc.Queries` كمان بيمنع التنفيذ إنه يعمل `Begin` تاني جوه transaction اللي بينادي.

### 3. `Core/JobQueue`
- `Enqueue(ctx, tx, Request) (Enqueued, error)`:
  - بيقبل `OriginKind = 'pipeline'` بس، ومعاه `OriginId` (الـ `RunJob` في M4.8)، ومين عمله. الـ jobs اليدوية (`manual`) بتيجي من `POST /jobs` بس.
  - نفس فحص `POST /jobs` بالظبط: `jobspec.Prepare`، والأسرار (`SECRETS_UNAVAILABLE` لحد M5)، والاسم، والأولوية، وحد الطابور (1000 لكل مؤسسة) تحت نفس الـ advisory lock. والكود نفسه مشترك (`insertJob`) عشان مايبقاش فيه طريقين بيتحققوا بشكل مختلف.
  - `RequiredFeatures` من قايمة معروفة بس (`checkout` و`plugins` و`artifacts` و`cache`)، ومن غير تكرار ومرتّبة.
  - **الملكية جوه الـ port نفسه** (بعد المراجعة الأمنية): اللي عمل الـ job لازم يبقى عضو حالي في المؤسسة وحسابه مش موقوف، والـ `OriginId` لو اتستخدم قبل كده في job من مؤسسة تانية بيترفض. ده بديل الـ FK اللي مش موجود بين موديولين، والتحقق الكامل إن الـ `RunJob` نفسه من نفس المؤسسة بيعمله `Pipelines` في نفس الـ transaction لما الجدول ييجي في M4.8.
  - **الـ jobs اللي محتاجة ميزات ليها حد لوحدها:** 100 job مستني لكل مؤسسة، وبره حد الـ 1000 بتاع الـ jobs العادية. كده jobs مافيش runner يقدر ياخدها لحد M4.7 ماتملاش الطابور على الشغل اللي بيمشي النهارده. اخترنا ده بدل «مابتتعدّش خالص» لأن الحد لوحده أبسط من طابور مالوش سقف.
- **الـ audit مسؤولية اللي بينادي، وده جزء من عقد الـ port:** `Enqueue` و`Cancel` مابيكتبوش `job.created` ولا `job.cancel_requested`. أي موديول بينادي الـ port لازم يكتب حدث audit بتاعه بعد الـ commit (`Pipelines` بيسجّل الـ run والإلغاء)، والمراجعة بتتأكد من ده في كل مرحلة بتضيف نداء جديد.
- `Cancel(ctx, tx, Cancellation) (bool, error)`: نفس إلغاء ADR-0047 (الـ `queued` بيتلغي على طول، والشغّال بيتعلّم ويشوفه الـ runner في الـ heartbeat). idempotent، والـ job اللي خلص بيرجّع `false` من غير خطأ، لأن إلغاء run فيه jobs خلصت حاجة عادية.
- `jobs.Service` فيه method اسمها `Cancel` للـ API، فالتنفيذ type صغير (`jobs.queue`) بيرجع من `Service.Queue()`، و`App` بيحطه في `Wired.JobQueue`.

### 4. أعمدة `"Jobs"` الجديدة (migration `00041`)
- `"OriginKind"` (`manual` أو `pipeline`، والافتراضي `manual` للـ jobs القديمة)، و`"OriginId"`، وCHECK إن الـ `manual` مالوش origin والـ `pipeline` لازم ليه.
- `"OriginId"` **من غير FK**، لأن الـ `RunJob` في جدول موديول تاني (زي `"RoleBindings"."ProjectId"`). والـ index عليه للتاريخ («كل محاولات الـ RunJob ده»).
- `"RequiredFeatures"` (`TEXT[]` بـ CHECK على القايمة المعروفة). الـ lease بيدّي الـ job بس لو `"RequiredFeatures" <@ features`، والـ runner لسه مابيعلنش features (ده M4.7)، فالسيرفر بيبعت قايمة فاضية. يعني job محتاج ميزة **بيستنى ومابيروحش لـ runner قديم** (fail closed). عرض `NO_MATCHING_RUNNER` للحالة دي بييجي مع M4.7.

### 5. الصلاحيات على المشروع
- **قاعدة جديدة:** `policy.Project(method, path, action, orgParam, projectParam)` بـ `ScopeProject`. الـ guard بيتأكد من العضوية الأول (غير العضو 404 من غير ما يلمس قاعدة البيانات)، وبعدين بيسأل `auth.ProjectResolver` (اللي `Projects` بينفّذه): المشروع موجود في المؤسسة دي؟ وإيه الـ bindings على المشروع للمستخدم ده، مباشرة أو من فرقه؟ المشروع مش موجود أو في مؤسسة تانية: 404.
- **الدور على المشروع** (`policy.ProjectRole`):
  - لو فيه binding على المشروع، أعلى binding هو الدور على المشروع ده، **سواء أعلى أو أقل من دور المؤسسة**. فـ Developer بـ binding Maintainer بيدير المشروع ده بس، وMaintainer بـ binding Viewer بيقرا بس.
  - **الـ Owner والـ Admin عمرهم ما بيقلّوا.** هما اللي بيديروا الـ bindings أصلًا، فالتقليل كان هيبقى إحساس كاذب بالتقييد مش حدّ حقيقي.
  - الـ binding عمره ما بيدّي `owner`، وbinding من غير عضوية مابيدّيش حاجة.
  - مفيش binding: دور المؤسسة زي ما هو.
- `policy.Can` على resource فيه مشروع بيقبل إجراءات المشروع بس (`IsProjectAction`)، فـ binding على مشروع مايقدرش يفتح إجراء على مستوى المؤسسة.
- **الإنشاء والقايمة على مستوى المؤسسة:** `POST /projects` محتاج `project.manage` من دور المؤسسة نفسه، فالـ binding على مشروع مابيسمحش بعمل مشروع جديد.
- **`ForkPolicy = automatic`** (تشغيل PRs الـ forks من غير موافقة، السؤال 1) محتاج Admin على المشروع (`FORK_POLICY_NOT_ALLOWED` غير كده)، والرجوع لـ `approval` مسموح لأي حد بيدير المشروع لأنه أشد.
- الـ guard بيحط الدور على المشروع في `auth.Principal.ProjectRole`، فالـ service مابيحسبوش تاني.
- **الخطوة التانية (ADR-0054):** `project.manage` بقى من الإجراءات الحساسة، فالعضو اللي عدّى المهلة من غير خطوة تانية مابيعدّلش ولا بيعمل ولا بيأرشف مشروع، والقراية مسموحة. وقاعدة `admins` بتتحسب على **الدور على المشروع**: Developer اتربط Admin على مشروع بيتعامل كـ Admin في المشروع ده، فالـ membership بقى فيها تاريخ تاني (`AdminSecondFactorDue`) محسوب كأن الدور Admin، والـ policy بتستخدمه لما الـ binding يرفع الدور لـ Admin. ده اتلقط في المراجعة الأمنية (MEDIUM).

### 6. الإجراءات والتوكنات
- **الإجراءات بتيجي مع الـ routes بتاعتها.** M4.3 فيه `project.read` (كل الأدوار) و`project.manage` (Owner وAdmin وMaintainer) بس. و`git.connect` (M4.4) و`run.*` (M4.8) و`artifact.read` (M4.9) بيدخلوا مع الـ routes بتاعتهم بنفس جدول الأدوار في الخطة. السبب: اختبار الـ route-table بيفشل لو فيه إجراء مافيش route بيستخدمه، وده اختبار مهم يفضل صارم.
- **التوكنات:** `project.read` و`project.manage` بقوا scopes، والـ migration بيوسّع CHECK `"ApiTokens"."Scopes"` (والـ down بيشيلهم). عمل التوكن محدود بدور المؤسسة (Developer مايقدرش ياخد `project.manage` حتى لو عنده binding Maintainer)، ووقت الاستخدام الـ binding بيتطبّق برضه، فتوكن Maintainer اتقلّل لـ Viewer على مشروع مايعدّلهوش. كل مرحلة بتضيف scopes بـ migration بتاعها.

### 7. الـ hook
- `port_rules.py` في `.claude/hooks/dhow_hooks`:
  - الـ port بيستورد من السيرفر `Database/Sqlc` بس.
  - الـ port بيتستورد من الموديولات و`App` بس. أي package تاني في `Core` أو `Config` أو `Database` مرفوض.
  - مفيش `func` ولا `var` على مستوى الـ package في الـ port.
- `check_route_policies` بيقبل `policy.Project(` كـ policy معلنة.
- الاختبارات في `.claude/hooks/tests/test_port_hooks.py`، ومنها اختبار بيعدّي الـ ports الحقيقية في الريبو على الـ hook.

## اللي اتأجّل
- **إدارة الـ bindings على المشروع من الـ API:** `BindingScope` لسه `[org]`. الاختبارات بتزرع الـ bindings مباشرة. القرار (فرق `Identity` بـ `ProjectId`، أو routes تحت المشروع) مع الواجهة في M4.10.
- **`"CreatedByUserId"` في `"Jobs"` إجباري.** الـ jobs اللي جاية من webhook مالهاش مستخدم، وده يتحسم في M4.8 (مستخدم الـ trigger، أو العمود يبقى nullable بـ migration).
- **`POST /organizations/{id}/jobs`:** Sabry وافق إنه يتشال من الإنتاج (السؤال 2)، بس الخطة مابتحطّوش في M4.3، فلسه موجود.

## البدائل اللي اترفضت
- **outbox للـ EventBus:** بيحل الضياع، بس بيحوّل كل نداء بين موديولين لحدث async بـ worker وإعادة وidempotency، وده أعقد بكتير من نداء في نفس الـ transaction. والـ outbox للـ audit لسه مطلوب لوحده.
- **callbacks (`func` fields) بيركّبها `App` من غير port:** نفس الفكرة من غير اسم ولا مكان، فالـ hook مايقدرش يقفلها، والتوقيع بيتكرر في كل موديول.
- **موديول واحد كبير للـ pipelines والـ jobs:** بيكسر حدود M3 المختبرة (الـ lease والـ fencing والـ reconciler)، وبيكبّر الملفات فوق 500 سطر.

## العواقب
- **أي تواصل بين موديولين لازم يكون في نفس الـ transaction** بيعدّي على port، والـ EventBus بيفضل للحاجات اللي ضياعها مقبول (الـ audit والإشعارات).
- **الـ port مابيقدرش يخفي شغل تقيل:** بيشتغل جوه transaction اللي بينادي، فأي حاجة بطيئة (شبكة أو API مزوّد) لازم تبقى بره بـ worker (زي `CommitStatus` في M4.4).
- **كل route على مشروع بيعمل query واحدة زيادة** (المشروع والـ bindings مع بعض) بالـ index الجديد `IX_RoleBindings_Project`.
