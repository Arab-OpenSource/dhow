# ADR-0046: الـ logs: chunks متسلسلة من غير فراغات في جدول Postgres مقسّم بالأيام، وSSE بيكمّل من آخر seq

- **الحالة:** مقبول (قرار مفوَّض في M3)
- **التاريخ:** 2026-10-27
- **بيعدّل:** القسم 5 في `docs/architecture.md`: الـ segments في object storage اتأجلت لـ M4 مع الـ blob store.

## السياق
- `docs/architecture.md` قال إن الـ runner بيبعت `(job_id, seq, ts, stream, bytes)`، والسيرفر بيكتبهم في object storage كـ segments. والواجهة بتقرا التاريخ وبعدين بتتابع بـ SSE من آخر `seq`، من غير فراغات ولا تكرار.
- مفيش object storage في الكود لسه. الـ S3 والـ disk بيجوا مع الـ artifacts في M4.
- السيرفر ممكن يبقى أكتر من replica، فأي تخزين على قرص الـ instance مش هيشتغل.
- قاعدة المشروع: الحذف soft بس، وممنوع `DELETE FROM` في `Queries/`. والـ logs كبيرة ولازم تتشال فعلًا بعد مدة الاحتفاظ.

## القرار

### في الـ runner
- **التقسيم:** كل stream (`stdout` و`stderr` و`system`) بيتقسم سطور، والسطر بيعدّي على الـ masker ([ADR-0045](0045-secrets-transport-and-masking.md)). بعدها بيتجمّع في chunks: 64 KiB بالكتير، أو كل 250 ms، أيهما أقرب.
- **الترقيم:** كل chunk واخد `seq` من 1 ومن غير فراغات، لكل attempt.
- **الطابور:** في الذاكرة لحد 8 MiB، وبعدها ملف spill لحد 256 MiB بصلاحية `0600` في مجلد الـ attempt (فيه نص متقنّع بس، وبيتمسح في الهدم). ولما الاتنين يتملوا، الـ runner بيبطّل يقرا من الـ pipe، فالـ job بيستنى ([ADR-0042](0042-runner-protocol-and-leasing.md)).
- **الإرسال:** `LogService.Append` بـ batches لحد 1 MiB، ومعاها `lease_id` و`fencing_token`.

### في السيرفر
- **جدول `"LogChunks"` مقسّم بـ `PARTITION BY RANGE ("CreatedAt")`، partition لكل يوم UTC.**
- **الأعمدة:**
  - الأساسية الأربعة.
  - `"OrganizationId"` و`"JobId"` و`"AttemptId"` و`"Seq"` (`bigint`).
  - `"Stream"` (`TEXT` بـ CHECK).
  - `"StepIndex"` و`"EmittedAt"` (وقت الـ runner، للعرض بس).
  - `"Data"` (`bytea`، 65536 بالكتير).
  - المفتاح `PRIMARY KEY ("Id", "CreatedAt")`، لأن Postgres بيطلب إن مفتاح الجدول المقسّم يشمل عمود التقسيم.
- **من غير فراغات بقفل الـ attempt:** Postgres مابيقدرش يفرض unique على `("AttemptId", "Seq")` في جدول مقسّم بالوقت. فالـ `Append` بيعمل في transaction واحدة:
  1. يتحقق من الـ fencing.
  2. يقفل صف الـ attempt.
  3. لو أول `seq` في الـ batch أكبر من `"LastSeq" + 1`، يرفض بـ `expected_seq`.
  4. أي seq أصغر من `"LastSeq"` أو قده اتخزّن قبل كده، فيتجاهل.
  5. الباقي بيتكتب، و`"LastSeq"` و`"LogBytes"` بيتحدّثوا.
  
  ده بيدّي idempotency وترتيب من غير index فريد.
- **السقف:**
  - لكل attempt `DHOW_JOB_LOG_MAX_BYTES` (افتراضيًا 64 MiB، وبالكتير 1 GiB). أول batch يعدّي السقف بيتقبل منه اللي في الحد، ومعاه سطر `system` بيقول إن الـ log اتقص، والرد فيه `log_truncated`. والـ runner بعدها بيكمّل يقرا من الـ pipe ويرمي، عشان الـ job مايقفش.
  - وكمان 64 KiB للـ chunk، و1 MiB للطلب.
- **الـ partitions:**
  - موديول `Jobs` بيعمل partitions لـ 3 أيام قدّام، عند الإقلاع وكل ساعة، تحت `pg_advisory_lock`.
  - وبيعمل `DROP TABLE` للـ partition اللي أقدم من `DHOW_LOG_RETENTION_DAYS` (افتراضيًا 30، من 1 لـ 3650).
  - لو الـ partition بتاعة النهارده مش موجودة، الـ `Append` بيفشل `UNAVAILABLE` والـ runner بيعيد. ده خطأ تشغيل بيظهر في `/health/ready`، مش بيضيّع بيانات.
- **استثناء القاعدة:**
  - `"LogChunks"` بيانات تشغيل بالحجم، والاحتفاظ بيها بالوقت بس. فالحذف بـ `DROP` للـ partition كلها، مش `DELETE FROM` ولا soft delete.
  - الاستثناء ده للجدول ده بس، والـ hook بيتعدّل يقبل مفتاح مركّب فيه `"Id"` للجداول اللي فيها `PARTITION BY` بس، ومعاه اختبار.
  - `"Jobs"` و`"JobAttempts"` فاضلين soft delete.
- **الاحتفاظ لكل مؤسسة:** لو مؤسسة عايزة احتفاظ أقل من الـ instance، في M3 الـ logs الأقدم بتستخبى في القراية بس، وبتتمسح فعلًا مع الـ partition. الحذف الحقيقي لكل مؤسسة بييجي مع الـ segments في M4.

### القراية
- **التاريخ:** `GET /api/v1/organizations/{id}/jobs/{jobId}/logs?Attempt=&FromSeq=&Limit=` (`Limit` لحد 500 chunk، والـ query parameters بـ PascalCase زي `Page` و`Limit` الموجودين). الرد JSON فيه `Seq` و`Stream` و`StepIndex` و`EmittedAt` و`Text`. الـ `Text` UTF-8، وأي byte مش صالح بيبقى U+FFFD، والواجهة بتحوّل الـ ANSI لـ spans ومابتحطّش HTML أبدًا.
- **المتابعة الحية:** `GET /api/v1/organizations/{id}/jobs/{jobId}/logs/stream?Attempt=` بـ `text/event-stream`.
  - الاستكمال بـ `Last-Event-ID` (هو الـ `seq`) أو `FromSeq`.
  - الأحداث: `chunk` (الـ `id` هو الـ seq) و`status` (تغيّر حالة الـ attempt) و`end`.
  - comment كل 15 ثانية عشان الـ proxies ماتقفلش الاتصال.
  - **المصادقة:** بالـ session cookie، والـ policy `log.read` وقت الفتح. ومع كل comment بنشيّك على الجلسة تاني، فالجلسة الملغية بتقفل الـ stream في 15 ثانية بالكتير.
- **التوزيع على أكتر من instance:** `NOTIFY dhow_log_appended, '<attemptId>:<lastSeq>'` بعد الـ commit. وكل instance فيها اتصال `LISTEN` واحد وhub في الذاكرة، وكل مشترك بيقرا الصفوف اللي بعد آخر seq بعته. لو `NOTIFY` ضاع، الـ comment الدوري بيعمل قراية احتياطية. Redis بيستبدل الـ NOTIFY بعدين ([ADR-0006](0006-postgres-default-redis-optional.md)).
- **الحدود:**
  - 10 streams لكل جلسة، و2000 لكل instance (`DHOW_SSE_MAX_STREAMS`).
  - كل مشترك ليه buffer محدود. المشترك البطيء بيتقفل، والمتصفح بيرجع من آخر `Last-Event-ID`، فمحدش بيوقف التوزيع على الباقيين.
- **الـ timeout:** المسارين دول بره الـ middleware بتاع `config.APITimeout`، وليهم write deadline بيتمد مع كل كتابة بـ `http.ResponseController`.

## البدائل اللي اتدرست
- **Postgres large objects:**
  - مالهاش partitions، ومحتاجة `vacuumlo`.
  - الـ backup بتاعها مختلف، والـ soft delete مش ممكن.
  - القراية من seq معيّن صعبة.
- **ملفات على القرص بـ offsets:** بتكسر مع أكتر من replica ومع Kubernetes من غير قرص مشترك.
- **object storage على طول:** S3 مافيهوش append، وكل chunk هيبقى object (بطء وتكلفة). هو المكان الصح للـ logs المقفولة، وده M4: compaction لكل attempt بعد ما يخلص لـ segment واحد وindex للـ offsets. وساعتها الـ partitions تقدر تتشال بعد يومين بدل 30.
- **جدول من غير تقسيم ومسح بـ `DELETE`:** بيكسر القاعدة، وبينفخ الجدول وبيحمّل الـ vacuum.

## العواقب
- الـ logs في M3 بتكبّر قاعدة البيانات والـ backup لحد ما الـ segments تيجي في M4، والوثائق بتقول كده. الـ TOAST بيضغط النص (lz4).
- **الـ hook:** قاعدة الأعمدة الأساسية بتقبل مفتاح مركّب فيه `"Id"` للجداول المقسّمة، واختبار جديد في `.claude/hooks/tests/`.
- **الـ reverse proxy:** محتاج `proxy_buffering off` على `/api/v1/organizations/*/jobs/*/logs/stream`. ده بيتضاف لـ `docs/install/reverse-proxy.md` مع M3.4.
- الـ partition الناقصة بتبان في `/health/ready` قبل ما تبقى مشكلة.
