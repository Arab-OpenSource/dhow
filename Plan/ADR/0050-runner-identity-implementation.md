# ADR-0050: تنفيذ هوية الـ runner في M3.2: صيغة الرسائل الموقّعة، والـ gate، والـ throttle، وكشف النسخ

- **الحالة:** مقبول (قرار مفوَّض في M3.2)
- **التاريخ:** 2026-10-28
- **بيكمّل:** [ADR-0041](0041-runner-identity-and-registration.md) و[ADR-0042](0042-runner-protocol-and-leasing.md) و[ADR-0035](0035-identity-core-implementation.md) و[ADR-0047](0047-job-state-machine-and-reconciliation.md).

## السياق
ADR-0041 حدّد المفاتيح والتوكنات والـ challenge، وADR-0047 حدّد `ScopeRunner` وأنواعه. التنفيذ طلّع أسئلة صغيرة لازم تتقفل عشان M3.3 لحد M3.5 يبنوا عليها من غير ما يرجعوا يفتحوها: الرسائل الموقّعة بتتبني بالظبط إزاي، و«origin السيرفر» يعني إيه لما الـ runner plane على listener لوحده، والـ throttle بيتعد على إيه، وإيه اللي بيحصل للـ runner اللي اتقفل بسبب نسخ هويته.

## القرار

### الرسائل الموقّعة
- في package مشترك `internal/runnersig`، عشان السيرفر والـ runner (M3.5) والـ smoke يبنوا نفس الـ bytes.
- **الصيغة:** الـ label، وبعده كل حقل بطوله (uint32 big-endian) وبعدين الـ bytes. والأرقام (رقم مفتاح السيرفر وإصدار مفتاح الـ runner) 4 bytes من غير طول. كده مفيش حقلين ممكن يتلزقوا في رسالة تانية بنفس الـ bytes.
- **الـ labels:** `dhow/runner-register/v1` و`dhow/runner-auth/v1` و`dhow/runner-rotate-keys/v1`.
- **رسالة `RotateKeys`** فيها الـ origin و`RunnerId` والإصدار الحالي والمفتاحين الجداد، وبيوقّعها المفتاح القديم والجديد على نفس الـ bytes.

### الـ origin
- هو origin بتاع `DHOW_PUBLIC_URL` دايمًا، حتى لو الـ runner plane على `DHOW_RUNNER_LISTEN_ADDR`.
- السبب: الـ listener المنفصل مالوش عنوان عام متعرّف في السيرفر، والـ origin وظيفته يربط التوقيع بالتثبيت ده بالظبط، مش بالعنوان اللي الـ runner بيكلمه. الـ runner في M3.5 بيستخدم origin بتاع `DHOW_RUNNER_URL` افتراضيًا، ومعاه إعداد يغيّره لما الـ listener منفصل.

### مفاتيح السيرفر
- `Core/RunnerAuth.DeriveKeys` بيشتق بـ HKDF-SHA256 من `DHOW_SECRET_KEY` (من خلال `Core/Crypto.DeriveKey`) تلات مفاتيح بـ labels منفصلة:
  - `dhow/runner-plane-signing/v1`: seed لمفتاح Ed25519، ورقم إصداره 1.
  - `dhow/runner-registration-token/v1`: HMAC-SHA256 لتوكن التسجيل.
  - `dhow/runner-challenge/v1`: HMAC-SHA256 للـ challenge.
- الـ access token بيتخزّن SHA-256 من غير مفتاح، زي جلسات المتصفح، لأنه عشوائي 32 byte.

### الـ gate
- `Gate.RequestGate(table)` بيرجّع `connect.WithRequestGate`، يعني الفحص بيحصل قبل قراءة الـ body.
- **الترتيب:**
  1. صف الـ policy: procedure من غير صف `ScopeRunner` صالح بيرجّع `PERMISSION_DENIED` ويتسجّل خطأ.
  2. `Dhow-Protocol: 1` مطلوب في كل procedure. الغايب أو المختلف بيرجّع `FAILED_PRECONDITION` بـ `RUNNER_UPGRADE_REQUIRED`.
  3. أي cookie من cookies المتصفح (`__Host-dhow_session` و`__Host-dhow_mfa`) بيرجّع `UNAUTHENTICATED` في كل procedure، حتى `Register` و`Challenge`. فمفيش طلب على الـ runner plane بيشيل جلسة متصفح.
  4. `RunnerSession` بس هو اللي بيقرا `Authorization: Bearer dhow_rat_...`. الـ runner الملغي بيرجّع `PERMISSION_DENIED` بـ `RUNNER_REVOKED` حتى لو جلسته اتقفلت، والجلسة المنتهية أو اللي اتبدّلت بترجّع `UNAUTHENTICATED`.
- رسالة الخطأ في Connect هي الكود الثابت بس (`INVALID_REGISTRATION_TOKEN`، `VALIDATION_FAILED: Name`...). مفيش ترجمة، لأن اللي بيقرا آلة. وأي خطأ مش متوقّع بيتسجّل في الـ log ويرجع `INTERNAL` من غير تفاصيل (interceptor في `Core/RunnerAuth`).
- **اتجاه الاستيراد:** `Core/RunnerAuth` بيستورده `App` و`Modules/Runners` و`Modules/Jobs` بس. و`Core/Auth` و`Core/RunnerAuth` مابيستوردوش بعض. والـ hook بيفرض الاتنين.

### التسجيل
- **الترتيب:** حجز الـ throttle، وبعدين شكل التوكن، وبعدين التحقق من الحقول والمفاتيح والـ proof، وبعدين استهلاك التوكن وإضافة الـ runner في transaction واحدة.
- أي خطأ في الحقول أو الـ proof بيرجّع `INVALID_ARGUMENT` قبل ما التوكن يتلمس، فالتوكن مابيتحرقش بطلب غلط، ومابيكشفش حاجة عن التوكن.
- مفتاح توقيع اتسجّل قبل كده (حتى لو الـ runner اتشال soft) بيرجّع `ALREADY_EXISTS` بـ `RUNNER_KEY_IN_USE`، والـ transaction بترجع، فالاستخدام مابيضيعش. ده بيوصل لحامل توكن صالح بس.
- التوكن المنتهي أو المستهلك أو الملغي أو بتاع مؤسسة اتشالت أو اللي مااتعملش أصلًا أو بشكل غلط: كلهم `UNAUTHENTICATED` بـ `INVALID_REGISTRATION_TOKEN` بنفس الرد بالظبط.
- **الـ labels:**
  - labels الأدمن (من التوكن، وبعدين `PATCH`) بصيغة `key` أو `key=value` بحروف صغيرة، وبحد 32، ومتربة ومن غير تكرار، والـ prefix `dhow.` ممنوع فيها.
  - labels القدرات بتتولد من `Os` و`Arch` ومستويات العزل بس (`dhow.os=linux` و`dhow.arch=amd64` و`dhow.isolation=container`)، في عمود لوحده (`"CapabilityLabels"`).
  - الـ OS المسموح في M3 `linux` بس، والـ arch `amd64` و`arm64`.

### الـ throttle
- جدول `"RunnerThrottles"` بنفس شكل `"LoginThrottles"` وبنفس الحجز قبل التحقق.
- **المفتاح بالـ IP بس**، لـ `Register` (`register:ip:<ip>`) ولـ `Authenticate` (`authenticate:ip:<ip>`)، و20 محاولة ببلاش لكل واحد.
- مفيش مفتاح بالـ runner، لأن `RunnerId` مش سر (أي عضو في المؤسسة شايفه)، فمفتاح بيه كان هيخلّي أي حد يقفل runner شغّال. والتخمين أصلًا مش ممكن مع Ed25519، والـ throttle هنا ضد التكلفة.
- النجاح بيرجّع المحاولة اللي اتعدت بس، فأسطول ورا IP واحد مابيتقفلش، والفشل مابيتمسحش بنجاح.
- أثناء القفل الرد هو نفس الخطأ العام، ومعاه `Retry-After` في الـ metadata، زي [ADR-0035](0035-identity-core-implementation.md).

### الجلسة وكشف النسخ
- الـ challenge مش بيقرا قاعدة البيانات خالص، وبيرجع لأي UUID، فمفيش طريقة تعرف بيها لو runner موجود.
- `Authenticate` بيقفل صف الـ runner (`FOR UPDATE`)، ويلغي الجلسة اللي قبلها، ويضيف الجديدة بـ `ON CONFLICT ("ChallengeNonce") DO NOTHING`. لو مفيش صف رجع، يبقى الـ challenge اتعاد، والـ transaction كلها بترجع.
- **التبديل** هو جلسة جديدة لغت جلسة لسه صالحة (`"ReplacedSessionId"`). لو التبديلات في آخر 10 دقايق عدّت 3، الـ runner بيبقى `disabled` بسبب `clone_suspected`، وجلساته بتتلغي، وبيتسجّل `runner.disabled_clone_suspected`، والرد `PERMISSION_DENIED` بـ `RUNNER_DISABLED`.
- التجديد كل 10 دقايق (ثلثين الـ 15) تبديل واحد كل مرة، فمابيوصلش للحد. والرستر بعد ما الجلسة تنتهي مش تبديل.
- **الـ runner اللي اتقفل بسبب النسخ مابيقدرش يعمل `Authenticate`** لحد ما الأدمن يرجّعه بـ `PATCH {"Disabled": false}` أو يلغيه. السبب إن الهوية نفسها بقت مشكوك فيها، فمفيش نسخة منهم نثق فيها. أما التعطيل اليدوي فبيسيب الـ runner يتصادق عشان يخلّص الشغّال (ADR-0041).

### الإلغاء والتعطيل من الـ API
- `revoke` على الـ runner نهائي وidempotent: أول مرة بيغيّر الحالة ويلغي الجلسات وبيطلّع `runner.revoked`، والمرات اللي بعدها بترجّع نفس الـ runner من غير حدث.
- `PATCH` على runner ملغي بيرجّع `409` بـ `RUNNER_REVOKED`.
- إلغاء توكن التسجيل idempotent، والـ runners اللي اتسجّلت بيه قبل كده بتفضل شغّالة.

### الوقت
- زي M2.1، الأوقات في `Modules/Runners` من ساعة الـ instance اللي بتتحقن (`Now`)، مش من `now()` بتاع قاعدة البيانات. ده عشان الاختبارات تقدّم الساعة (انتهاء التوكن والـ challenge والجلسة). الـ leases في M3.3 بتستخدم `now()` بتاع قاعدة البيانات زي ADR-0042، لأن هناك اتنين instances ممكن يختلفوا على مدة الـ lease.

### الـ listener المنفصل
- لو `DHOW_RUNNER_LISTEN_ADDR` موجود، الـ runner plane بيتشال من الـ listener العام (`/runner-api/` هناك بيرجّع 404 بالـ envelope)، وبيتخدم على المسار نفسه `/runner-api/` في الـ listener التاني، ومفيش `/api/v1` عليه.
- الـ TLS مطلوب إلا مع `DHOW_DEV=true`. الـ config بيرفض شهادة من غير مفتاح، أو ملفات TLS من غير الـ listener، أو مفتاح صلاحياته مفتوحة لباقي المستخدمين، أو زوج مابيتحمّلش، أو نفس عنوان `DHOW_LISTEN_ADDR`.

## البدائل اللي اتدرست
- **الرسائل بفاصل (`‖` حرفيًا أو `\0`):** أبسط، لكن أي حقل فيه نفس الفاصل بيعمل لبس. الطول المسبوق مابيعتمدش على محتوى الحقل.
- **الرسائل كـ protobuf:** الـ serialization في protobuf مش مضمونة تبقى نفس الـ bytes بين الإصدارات واللغات، والتوقيع لازم على bytes ثابتة.
- **الـ origin من العنوان اللي الـ runner بيكلمه:** كان هيكسر الـ listener المنفصل ورا ingress، لأن السيرفر مايعرفش العنوان ده.
- **throttle بالـ runner كمان:** بيدّي أي عضو في المؤسسة طريقة يقفل بيها runner شغّال.
- **Challenge بيتأكد إن الـ runner موجود:** بيكشف مين موجود، ومحتاج قراية من قاعدة البيانات لطلب مش متوثّق.
- **الـ clone المقفول يكمّل يتصادق زي التعطيل اليدوي:** النسختين كانوا هيفضلوا يبدّلوا بعض ويشتغلوا، والمهاجم معاه جلسة.

## العواقب
- `internal/runnersig` بقى جزء من العقد بين السيرفر والـ runner. أي تغيير في الصيغة label جديد (`v2`)، مش تعديل.
- الـ runner اللي بيعيد التشغيل في loop سريع (أكتر من 3 مرات في 10 دقايق وجلسته لسه صالحة) هيتقفل كأنه نسخة. ده ثمن الكشف، و`doctor` في M3.10 بيبيّن السبب.
- `Dhow-Protocol` مطلوب من أول يوم، فأي client (حتى `buf curl`) لازم يبعته.
- الـ Dockerfile بقى بينسخ `packages/proto/gen/go`، لأن السيرفر بيستورده.
