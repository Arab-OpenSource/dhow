# ADR-0052: تنفيذ الـ logs في M3.4: الجدول المقسّم، والـ Append بقفل الـ attempt، والـ SSE بقراية لكل مشترك، والصيانة تحت advisory lock

- **الحالة:** مقبول (قرار مفوَّض في M3.4)
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0046](0046-job-logs-storage-and-streaming.md) و[ADR-0045](0045-secrets-transport-and-masking.md) و[ADR-0047](0047-job-state-machine-and-reconciliation.md) و[ADR-0051](0051-jobs-and-leasing-implementation.md).

## السياق
ADR-0046 حدّد شكل التخزين والقراية: `"LogChunks"` مقسّم بالأيام، وseq من غير فراغات بقفل الـ attempt، وSSE بـ `Last-Event-ID`، وNOTIFY بين الـ instances، والسقف والاحتفاظ. وADR-0051 أجّل سطر الـ `system` اللي بيتكتب لما `Complete` ييجي بعد انتهاء الـ lease. التنفيذ طلّع أسئلة لازم تتقفل قبل M3.5 (الـ runner) وM3.12 (الواجهة): الـ seq بتاع سطر السيرفر بيتاخد منين، والـ batch اللي فيه فراغ في النص بيتعامل إزاي، والصفحة بتتحدّ بإيه، والمشترك البطيء بيتعرف إزاي، والحدود لكل إيه، والـ partitions بتتعمل إزاي من غير SQL مبني كنص في Go.

## القرار

### قاعدة البيانات (`00032_job_logs.sql`)
- **`"LogChunks"`** بالأعمدة الأساسية الأربعة، و`"Id" uuid NOT NULL DEFAULT uuidv7()` مع `PRIMARY KEY ("Id", "CreatedAt")`، و`PARTITION BY RANGE ("CreatedAt")`. و`"Seq"` من 1، و`"Stream"` IN (`stdout`, `stderr`, `system`)، و`"StepIndex"` من 0 لـ 99 أو فاضي، و`"EmittedAt"` (وقت الـ runner، للعرض بس)، و`"Data"` من 1 لـ 65536 byte.
- **FK مركّب** (`"AttemptId"`, `"JobId"`, `"OrganizationId"`) على `"JobAttempts"`، ومعاه unique index جديد على نفس الأعمدة. يعني قاعدة البيانات نفسها بترفض chunk بمؤسسة أو job مش بتوع الـ attempt.
- **index** على (`"AttemptId"`, `"Seq"`) بيتعمل في كل partition. مفيش unique عليه لأن Postgres مايقدرش يفرضه في جدول مقسّم بالوقت، والترتيب بيتضمن بقفل الـ attempt.
- **عمودين جداد في `"JobAttempts"`:** `"LogTruncatedAt"` (اتقص إمتى) و`"LateResultLoggedAt"` (سطر النتيجة المتأخرة اتكتب إمتى).
- **الـ trigger بتاع الـ attempt اتعدّل** (`CREATE OR REPLACE` في نفس الـ migration، والـ `down` بيرجّع نسخة 00031 بالظبط):
  - في أي تعديل: `"LastSeq"` و`"LogBytes"` بيزيدوا بس، و`"LogTruncatedAt"` و`"LateResultLoggedAt"` بيتحطّوا مرة واحدة ومابيتغيّروش.
  - الـ attempt اللي خلص يقدر يزوّد `"LastSeq"` و`"LogBytes"` ويحط `"LateResultLoggedAt"`، عشان سطر النتيجة المتأخرة بييجي غالبًا بعد ما الـ reconciler قفل الـ attempt. أي عمود تاني فاضل مقفول زي ADR-0051.
- **الـ partitions من functions في الـ migration** مش من Go، عشان الـ DDL محتاج اسم متغيّر وممنوع نبني SQL كنص:
  - `"LogPartitionName"(date)` بيرجّع `LogChunks_YYYYMMDD`، و`"LogPartitionBound"(date)` بيرجّع `YYYY-MM-DD 00:00:00+00` (صيغة ISO بتتقري صح مهما كان الـ `DateStyle`)، و`"LogPartitionToday"()` هو يوم UTC من `now()` بتاع قاعدة البيانات.
  - `"EnsureLogPartitions"(n)` بيعمل النهارده و`n` يوم قدّام (من 0 لـ 31) في نفس schema الجدول الأب، بـ `format('%I.%I')`.
  - `"DropLogPartitions"(days)` بيعمل `DROP` لأي partition متركّبة على `"LogChunks"` واسمها بالصيغة دي ويومها أقدم من النهارده ناقص `days`. يعني مع 30 يوم، الـ log بيعيش من 30 لـ 31 يوم. وأي جدول تاني اسمه شبهه بيفضل.
  - `"LogPartitionReady"()` بيقول الـ partition بتاعة النهارده متركّبة ولا لأ.
  - الـ migration نفسها بتنادي `"EnsureLogPartitions"(3)`، فقاعدة بيانات جديدة بتقدر تستقبل logs قبل أول صيانة.

### `LogService.Append`
- **الـ handler** على `/runner-api/` ورا نفس الـ gate، بصف `policy.Runner(..., RunnerSession)`، و`ReadMaxBytes` واحد ميجا (`LogAppendMaxBytes` = `RunnerBodyMaxBytes`، واختبار بيتأكد إنهم متساويين).
- **التحقق قبل أي قفل:** 1024 chunk بالكتير، وكل chunk `seq >= 1`، و`stream` معروف، و`data` من 1 لـ 64 KiB، و`step_index` من 0 لـ 99، و`emitted_at` صالح. أي مخالفة `INVALID_ARGUMENT` بـ `VALIDATION_FAILED: <Field>` للـ batch كله.
- **الـ transaction:** `LockActiveLease` (نفس فحص الـ fencing والـ runner و`ExpiresAt > now()` زي باقي الـ RPCs، وأي فرق `LEASE_LOST`)، وبعدين `FOR UPDATE` على صف الـ attempt. ترتيب الأقفال lease وبعدين attempt، زي ADR-0051. الـ Append شغّال في `leased` كمان، عشان سطور سحب الـ image.
- **الترتيب داخل الـ batch:** بيترتب بالـ seq، والمكرر جوه نفس الـ batch بيتشال (الأول هو اللي بيكسب).
- **قاعدة القبول** (فرق عن ADR-0046، اللي كان بيرفض الـ batch كله لو فيه فراغ):
  - أي seq أصغر من `"LastSeq"` أو قده بيتجاهل (مكرر).
  - الجزء المتصل من `"LastSeq" + 1` بيتقبل.
  - أول فراغ بيوقف القراية، والرد فيه `expected_seq` = `last_seq + 1`. فلو أول seq في الـ batch بعد الفراغ، مفيش حاجة بتتكتب، زي ADR-0046 بالظبط. ولو الفراغ في النص، الجزء اللي قبله مابيتعادش.
- **`"LastSeq"` هو أعلى seq اتأكد** (من الـ runner أو من السيرفر)، مش أعلى seq اتخزّن:
  - سطر السيرفر (`LOG_TRUNCATED` أو `LATE_RESULT`) بياخد `"LastSeq" + 1`، فالـ seq فاضل فريد في الـ attempt من غير ما الـ runner يعرف حاجة.
  - بعد القص، الـ chunks اللي بتيجي بتتأكد (الـ seq بيزيد) ومابتتخزّنش. فـ `last_seq` في الرد ممكن يبقى أكبر من آخر seq الـ runner بعته بواحد (سطر الـ system). ده عقد M3.5: بعد `log_truncated` الـ runner بيكمّل يقرا الـ pipe ويرمي، ومابيعتبرش ده خطأ.
- **بعد الكتابة:** `NOTIFY dhow_log_appended, '<attemptId>:<آخر seq اتخزّن>'` في نفس الـ transaction، فبيوصل بعد الـ commit بس.
- **الـ partition الناقصة:** الإدخال بيفشل بـ `23514` من غير اسم constraint وعلى جدول `LogChunks`، فبيرجع `UNAVAILABLE` بـ `LOG_STORAGE_UNAVAILABLE`، والـ runner بيعيد نفس الـ seq لما الـ partition ترجع. والـ log فيه الـ attempt والخطأ بس، والخطأ ده مافيهوش البيانات (الـ detail بتاعه فيه `"CreatedAt"` بس).
- **الـ Append مابيمدّش الـ lease:** ده شغل `Heartbeat`. لكن الرد فيه `control` بطلب الإلغاء، زي ADR-0047.

### السقف
- **`DHOW_JOB_LOG_MAX_BYTES`** (افتراضيًا 64 MiB، من 1 MiB لـ 1 GiB، رقم صحيح من غير وحدة) على بيانات الـ runner لكل attempt.
- **الـ chunk اللي بيعدّي السقف بيتقص بالـ byte** عند الحد بالظبط، وبعده سطر `system` واحد: `dhow: LOG_TRUNCATED: ...`، والرد فيه `log_truncated`. وأي batch بعد كده بيرجع `log_truncated` من غير كتابة.
- **سطور السيرفر مش جوه السقف:** بتتحسب في `"LogBytes"` لكن مابتتمنعش بيه. هي سطرين بالكتير لكل attempt (قص ونتيجة متأخرة)، وكل واحد مرة واحدة.
- **القص بالـ byte ممكن يقسم حرف UTF-8:** النص بيبان بـ U+FFFD في الآخر، والـ bytes متخزّنة زي ما هي.

### سطر النتيجة المتأخرة (المتأجّل من ADR-0051)
- `Complete` بعد انتهاء الـ lease (الـ lease منتهي والـ attempt لسه مفتوح، أو الـ reconciler قفله من غير digest) بيرجّع `LEASE_LOST` زي ما هو، ومن غير ما يغيّر أي حالة.
- **بعد الـ commit**، في transaction لوحدها: قفل الـ attempt، ولو `"LateResultLoggedAt"` فاضي، سطر `system` بـ `dhow: LATE_RESULT: the runner reported <state> (exit code N, reason R) after its lease ended, ...` في `"LastSeq" + 1`.
- **مرة واحدة لكل attempt:** runner بيعيد `Complete` مايقدرش يملا الـ log. والإثبات إنه هو صاحب الـ lease هو نفس فحص `LockLeaseForCompletion` (الـ id والـ fencing والـ runner)، فـ runner تاني مايقدرش يكتب السطر.
- لو الكتابة فشلت (partition ناقصة مثلًا)، بيتسجّل تحذير والرد فاضل `LEASE_LOST`. وسطر log السيرفر بتاع ADR-0051 فاضل كمان.
- **نص سطور السيرفر بالإنجليزي** وبيبدأ بكود ثابت (`LOG_TRUNCATED` و`LATE_RESULT`). الـ log بيانات تشغيل زي الـ terminal مش نص واجهة، والواجهة (M3.12 أو M7) تقدر تترجم بالكود.

### الصيانة والجاهزية
- **`MaintainLogPartitions`** في transaction واحدة:
  1. `pg_try_advisory_xact_lock('dhow.log-partitions')`. لو instance تانية ماسكاه، الدورة بتتخطّى (`Skipped`) بدل ما تستنى. ده فرق عن `pg_advisory_lock` في ADR-0046: الانتظار كان ممكن يوقّف الإقلاع.
  2. `lock_timeout` 5 ثواني جوه الـ transaction، لأن الـ DDL بياخد قفل قوي على الجدول الأب، ومن غير timeout أي استنى وراه بيوقّف كل الـ Appends.
  3. `"EnsureLogPartitions"(3)` وبعدين `"DropLogPartitions"(DHOW_LOG_RETENTION_DAYS)`.
- **عند الإقلاع** قبل الـ reconciler، ولو فشلت السيرفر مابيقومش (fail closed، زي الـ reconciler). وبعدين كل ساعة في الخلفية، والفشل بيتسجّل والدورة الجاية بتعيد.
- **`/health/ready`:** فحص جديد `log_partitions` بيسأل `"LogPartitionReady"()`، ولو مش موجودة الرد `503`. الفحوصات الإضافية بتتركّب من `App` في `system.Deps.Checks` (موديول `System` مابيستوردش `Jobs`)، وبتتخطّى لو الـ migrations مش كاملة. والواجهة بتعرض الفحص الجديد في صفحة حالة السيرفر بنصوص عربي وإنجليزي.
- **`DHOW_LOG_RETENTION_DAYS`** من 1 لـ 3650 (افتراضيًا 30).

### القراية (`GET .../jobs/{jobId}/logs`)
- `log.read` (كل الأدوار ما عدا الـ Viewer، ADR-0047). الـ job لازم في المؤسسة اللي في المسار وإلا `404 NOT_FOUND`، والـ attempt (`Attempt`، أو الأخير من غيره) لازم يكون موجود وإلا `404 ATTEMPT_NOT_FOUND`.
- **الصفحة:** من `FromSeq` (افتراضيًا 1)، و`Limit` من 1 لـ 500 (افتراضيًا 100)، و**سقف حوالي 1 MiB** للصفحة (الصفوف لحد ما المجموع قبل الصف يوصل 1 MiB، فالصفحة دايمًا فيها صف واحد على الأقل). من غيره 500 chunk بـ 64 KiB كانوا هيبقوا 32 MiB JSON.
- **الحجم من غير فك البيانات:** `sum(octet_length("Data"))` بـ window في CTE بيقرا الطول من الـ TOAST pointer، وبعدين join بالمفتاح يجيب البيانات للصفوف اللي في الصفحة بس.
- **التقسيم بيتقص بالوقت:** الشرط `"CreatedAt" >= "LeasedAt"` بتاع الـ attempt بيخلّي Postgres يقرا الـ partitions من يوم الـ lease بس.
- **الرد:** `Attempt` و`AttemptId` و`State` و`LastSeq` و`Truncated` و`Chunks`، و`NextSeq` لو آخر chunk في الصفحة أقل من `LastSeq`.
- **`Text`:** الـ bytes كـ UTF-8، وكل byte مش صالح بيبقى U+FFFD لوحده.

### الـ SSE (`GET .../jobs/{jobId}/logs/stream`)
- **الـ route من الـ generated strict handler** (الـ hook بيمنع تسجيل route بإيدينا): الـ controller بيرجّع object بيعمل `VisitStreamJobLogsResponse`، والـ stream كله جوه الـ Visit.
- **الاستكمال:** `Last-Event-ID` (header، رقم من 0) وإلا `FromSeq`، والـ `id` بتاع كل حدث هو الـ seq.
- **الأحداث:** `status` (أول حاجة ومع أي تغيير في حالة الـ attempt أو القص)، و`chunk`، و`end` بعد آخر chunk لـ attempt خلص، وبعده الـ stream بيتقفل. والـ data JSON بأسماء PascalCase زي `JobLogChunk` في `openapi.yaml`، فمفيش newline ممكن يكسر صيغة الحدث.
- **الـ stream لـ attempt واحد:** لو الـ job اتعاد، الـ attempt القديم بيخلص (`end`)، والواجهة بتفتح الـ attempt الجديد. ومفيش attempt خالص: `404 ATTEMPT_NOT_FOUND`.
- **`retry: 3000`** في الأول، و`X-Accel-Buffering: no` (nginx بيحترمه لوحده)، و`Cache-Control: no-store`.
- **الوقت:**
  - routes الـ `Jobs` اتشالت من مجموعة `middleware.Timeout` في `App`، والموديول بيحطّ `RouteTimeout` لكل route بتاعه ما عدا الـ stream، ومعاه اختبار بيتأكد إن كل route تاني ليه deadline.
  - الـ stream بيشيل الـ read deadline بتاع الاتصال (`ReadTimeout` كان هيلغي الـ context بعد 30 ثانية)، وبيحط write deadline 10 ثواني مع كل كتابة بـ `http.ResponseController`.
- **فحص الجلسة كل 15 ثانية** مع الـ comment، بـ `Guard.Reauthorize` الجديد في `Core/Auth`: نفس الـ resolver ونفس `policy.Can`. فالجلسة الملغية أو الدور اللي نزل لـ Viewer أو العضوية اللي اتشالت بتقفل الـ stream في 15 ثانية بالكتير. وخطأ في قاعدة البيانات وقت الفحص بيقفل الـ stream كمان (fail closed)، والمتصفح بيرجع.
- **إيقاف السيرفر:** `Drain` بيقفل كل الـ streams على طول، عشان `Shutdown` مايستناش 20 ثانية والمتصفح يرجع على instance تانية.

### التوزيع والحدود
- **اتصال `LISTEN` واحد لكل instance للقناتين** (`dhow_jobs_queued` و`dhow_log_appended`)، نفس الـ hub بتاع ADR-0051. بعد أي اتصال جديد كل المشتركين بيصحوا لأن الإشعارات اللي فاتت ضاعت. و`finalize` بيبعت `NOTIFY` كمان، فحدث `status` و`end` بيوصلوا على طول.
- **مفيش buffer مشترك:** كل stream ليه channel صحيان بسعة 1 (الصحيان بيتجمّع والـ hub عمره ما بيستنى)، وبيقرا صفوفه هو بعد آخر seq بعته، بـ batches لحد 64 صف أو 256 KiB. فالذاكرة لكل stream محدودة، والـ stream البطيء مابيأثرش على حد.
- **المشترك البطيء:** كتابة ماخلصتش في 10 ثواني (الـ socket buffer اتملى) بتقفل الـ stream، ومعاها سطر `info` فيه `slow_client`. والمتصفح بيرجع بـ `Last-Event-ID`. ده تفسير «الـ buffer المحدود» في ADR-0046: الـ buffer هو الـ socket، والحد هو الـ write deadline.
- **القراية الاحتياطية:** مع كل comment (15 ثانية) الـ stream بيقرا تاني حتى لو مفيش إشعار.
- **الحدود في كل instance:** 10 streams **لكل مستخدم** (فرق عن «لكل جلسة» في ADR-0046: الجلسة الجديدة بتتعمل بتسجيل دخول تاني، فحد الجلسة كان سهل يتلف حواليه)، و`DHOW_SSE_MAX_STREAMS` (افتراضيًا 2000، من 1 لـ 100000) للـ instance. الزيادة `429 TOO_MANY_STREAMS` بـ `Retry-After: 5`، والحجز بيتعمل قبل ما الـ stream يبدأ وبيتفك مع نهايته.

### الأسرار والـ masking
- **الـ masking في الـ runner بس** (ADR-0045): قبل الـ spill وقبل الإرسال. السيرفر بيخزّن الـ chunks زي ما وصلت ومابيعملش masking تاني، لأنه كان هيحتاج القيم الصريحة طول مدة الـ job.
- **السيرفر مابيكتبش محتوى أي log في log بتاعه:** الأسطر فيها IDs وأكواد وأخطاء بس، واختبار بيبعت علامة فريدة في الـ log ويتأكد إنها مش موجودة في log السيرفر، حتى في مسار الـ partition الناقصة.

### الـ reverse proxy
- **nginx:** الـ `proxy_buffering off` والـ timeout ساعة على مسار الـ stream بس (location بـ regex)، بدل `/api/v1/` كله، فباقي الردود فاضلة بالـ buffering العادي. و`/runner-api/` ليه 90 ثانية للـ long-poll (25 ثانية) ورفع الـ logs.
- **Caddy:** matcher `@logstream` بـ `flush_interval -1` و`read_timeout 1h`، والباقي `read_timeout 90s`.
- **Traefik:** من غير تغيير، لأنه مابيعملش buffer ومفيش timeout على رد Dhow.
- **`run.sh`** بيتأكد إن `/runner-api/` بيوصل للـ runner plane، وإن مسار الـ stream بيوصل لـ Dhow بالـ cookie. واختبار Go جديد بيتأكد إن الـ fixtures فيها قاعدة الـ stream والـ timeout، غير اختبار التطابق مع الوثيقة.

### الـ hook
- **قاعدة الأعمدة الأساسية** بتقبل `"Id" uuid NOT NULL DEFAULT uuidv7()` مع `PRIMARY KEY ("Id", "<Column>")` بس لو `PARTITION BY` جاي بعد قوس الجدول على طول. ومن غيره الرفض زي الأول، ومعاه اختبارات إيجابية وسلبية (`PARTITION BY` ناقص، و`"Id"` مش أول المفتاح، و`DEFAULT uuidv7()` ناقص، وعمود أساسي تاني ناقص).
- **قاعدة اسم الجدول** بتقبل `%I` و`%I.%I` في `CREATE TABLE`، لأنهم identifiers بيتحطّوا بين علامات تنصيص وقت التشغيل بـ `format()`. و`%s` واسم من غير تنصيص فاضلين مرفوضين، ومعاهم اختبار.

## قيود معروفة
- **الحدود والـ fan-out لكل instance:** مستخدم على 3 instances يقدر يفتح 30 stream. الحد على مستوى الـ cluster محتاج Redis أو جدول، وده مع HA (Business).
- **مفيش rate limit على `Append`:** كل نداء transaction قصيرة، والسقف بيحدّ التخزين. runner مسروق التوكن بتاعه يقدر يملا الـ log لحد السقف بس، وده نفس قيد إثبات الامتلاك في ADR-0051.
- **seq مكرر ببيانات مختلفة** في batch تاني: الأول اللي اتخزّن هو اللي بيكسب، ومفيش مقارنة. المقارنة كانت هتحتاج قراية كل chunk قديم.
- **chunk بيقسم حرف UTF-8:** كل chunk بيتحوّل لوحده، فالحرف المقسوم بيبان U+FFFD مرتين. الـ chunker في M3.5 لازم يقسم على حدود الحروف.
- **الاحتفاظ لكل مؤسسة:** مش في M3.4 (مفيش إعداد ليه لسه)، وبييجي مع الـ segments في M4.
- **الـ DDL بياخد قفل على `"LogChunks"`:** مرة في اليوم للـ partition الجديدة ومرة للحذف، ومحدود بـ `lock_timeout`.
- **قاعدة بيانات للقراية بس وقت الإقلاع** بتوقّف الإقلاع (fail closed). نفس الحالة وقت التشغيل بترجّع `503` و`UNAVAILABLE` من غير ما توقّف السيرفر.

## البدائل اللي اتدرست
- **رفض الـ batch كله لو فيه فراغ (ADR-0046 بالحرف):** كان هيخلّي الـ runner يعيد الجزء المتصل اللي اتبعت، والقبول الجزئي أسرع ونتيجته واحدة.
- **seq منفصل لسطور السيرفر:** كان هيحتاج عمود تاني ومفتاح ترتيب من اتنين في القراية والـ `Last-Event-ID`.
- **`pg_advisory_lock` بالانتظار:** instance بتقوم ورا instance بتعمل صيانة كانت هتستنى، والـ `try` كفاية لأن الشغل idempotent.
- **partition `DEFAULT`:** كانت هتخلّي الـ partition الناقصة مشكلة صامتة، وأي partition جديدة بعدها كانت محتاجة scan للـ default.
- **fan-out في الذاكرة بـ buffer لكل مشترك:** بيوفّر قراية قاعدة البيانات، لكن محتاج تسليم بين التاريخ والبث الحي وفحص فراغات. والقراية لكل مشترك هي اللي في ADR-0046، وأبسط وصحيحة من غير حالة.
- **حد لكل جلسة:** اتشرح فوق.
- **`proxy_buffering off` على `/api/v1/` كله (الإعداد القديم):** كان بيشيل الحماية من العملاء البطيئين عن كل الردود عشان route واحد.
- **SSE بـ handler مسجّل بإيدينا بره الـ generated code:** الـ hook بيمنعه، والـ route كان هيبقى من غير عقد في `openapi.yaml`.

## العواقب
- **عقد M3.5:**
  - الـ runner بيبعت batches متصلة.
  - مع `expected_seq` بيبدأ منه.
  - مع `UNAVAILABLE` بيعيد نفس الـ batch.
  - بعد `log_truncated` بيرمي ومابيبعتش.
  - بيقسم على حدود حروف UTF-8.
- **M3.12** بيفتح الـ stream لـ attempt واحد، ولما يجي `end` بيقرا الـ job ويفتح الـ attempt الجديد لو فيه.
- **الـ reverse proxy** لازم يتظبط تاني عند المستخدمين اللي عملوا إعداد nginx أو Caddy القديم (قبل الإصدار الأول، فمفيش حد).
- **`log.read` بقى إجراء في `Core/Policy`**، ومعاه صفوف الـ route-table لكل دور، والـ Viewer ممنوع.
- **مصطلح جديد في `Plan/Glossary.md`:** «سجل التشغيل» للـ job log، عشان نص الواجهة العربي مايبقاش فيه كلمة إنجليزي.
