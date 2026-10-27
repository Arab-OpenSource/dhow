# ADR-0041: هوية الـ runner وتسجيله: مفاتيح Ed25519 متثبّتة، وتوكن قصير، وإثبات امتلاك عند الحاجات الحساسة

- **الحالة:** مقبول (قرار مفوَّض في M3)
- **التاريخ:** 2026-10-27
- **بيكمّل:** [ADR-0005](0005-api-rest-openapi-and-connect.md) و[ADR-0007](0007-job-isolation.md) و[ADR-0035](0035-identity-core-implementation.md). الأرقام من 0037 لـ 0040 محجوزة لـ M2.

## السياق
- الـ runner بيستلم كود المستخدم وأسرار البيئة، فهويته أهم credential في المنصة بعد الـ KEK. لو اتسرقت، اللي سرقها بياخد jobs وأسرار.
- السيرفر بيشتغل ورا reverse proxy بيقفل الـ TLS (nginx أو Caddy أو Traefik، وأحيانًا Cloudflare قدّامهم)، وده المسار الموثّق في `docs/install/reverse-proxy.md`. يعني السيرفر نفسه مش شايف الـ TLS بتاع العميل.
- `docs/architecture.md` كان كاتب إن الـ runner بياخد Ed25519 keypair وتوكن قصير بيتجدد بـ challenge موقّع، والمطلوب نقرر بينه وبين mTLS بشهادات من CA داخلي.
- محتاجين اتجاهين: السيرفر يتأكد إن اللي بيكلمه هو الـ runner، والـ runner يتأكد إن الـ jobs والأسرار جاية من الـ control plane بتاعه هو، مش من أي حد في النص.

## القرار

### المفاتيح
- الـ runner بيولّد مفتاحين عنده، والمفتاح الخاص عمره ما بيخرج من الجهاز:
  - **مفتاح توقيع Ed25519:** هو الهوية.
  - **مفتاح HPKE من نوع MLKEM768-X25519 (X-Wing):** لاستلام الأسرار مشفّرة ليه هو بس ([ADR-0045](0045-secrets-transport-and-masking.md)). hybrid عشان يفضل آمن حتى لو اتسجّلت الحركة النهارده واتفكّت بعدين بحاسب كمّي.
- السيرفر بيثبّت (pin) المفتاحين العامّين وقت التسجيل في `"Runners"`، ومفتاح التوقيع عليه unique index، فنفس المفتاح ما يتسجّلش مرتين.
- **التخزين:**
  - على الـ host: `/var/lib/dhow-runner/identity` بصلاحية `0600` لـ user `dhow-runner`. ولو `systemd-creds` موجود، الـ installer بيشفّر الملف بـ `systemd-creds encrypt` (بـ TPM2 لو موجود) والخدمة بتقراه بـ `LoadCredentialEncrypted=`، فالمفتاح مش متخزّن صريح على القرص.
  - على Kubernetes: ملف في PVC الـ StatefulSet ([ADR-0048](0048-runner-packaging-and-kubernetes.md)).

### توكن التسجيل
- الأدمن بيعمله من الواجهة أو الـ API. شكله `dhow_rrt_` وبعده 32 byte عشوائي بـ base64url، وبيظهر مرة واحدة بس.
- بيتخزّن HMAC-SHA256 بمفتاح مشتق من `DHOW_SECRET_KEY` بـ HKDF (label `dhow/runner-registration-token/v1`)، زي نمط الـ API tokens.
- **محصور:**
  - في مؤسسة واحدة.
  - ومعاه الـ labels اللي الأدمن حددها. الـ runner مش بيقدر يختار labels لنفسه، لأن الـ labels هي اللي بتربط الـ runner بالبيئات (runner بيقول عن نفسه `production` يبقى ثغرة). الـ runner بيبلّغ بس عن قدراته (`os` و`arch` ومستويات العزل)، ودي بتتسجّل كـ labels أوتوماتيك بـ prefix محجوز (`dhow.os=linux`).
- **عمره:** ساعة افتراضيًا، و24 ساعة بالكتير.
- **الاستخدام:** مرة واحدة افتراضيًا. الأدمن يقدر يرفعه لحد 50 استخدام لأسطول (StatefulSet على Kubernetes مثلًا)، وكل استخدام بيعمل runner منفصل بهوية منفصلة. الاستهلاك ذرّي: `UPDATE ... SET "Uses" = "Uses" + 1 WHERE "Uses" < "MaxUses" AND "ExpiresAt" > now() AND "RevokedAt" IS NULL`.
- بيوصل للـ runner من ملف أو stdin (`--token-file`)، مش من argv ولا من سطر أوامر بيتحفظ في الـ history.
- الأدمن يقدر يلغيه قبل ما يخلص، وكل عملية عليه بتتسجّل في الـ audit.

### التسجيل (`RegistrationService.Register`)
- الـ runner بيبعت التوكن والمفتاحين العامّين وتوقيع بمفتاحه الخاص على `"dhow/runner-register/v1" ‖ origin السيرفر ‖ المفتاحين` (إثبات إنه بيملك المفتاح الخاص، ومش بيعيد تسجيل مفتاح حد تاني).
- الرد فيه: `RunnerId` و`OrganizationId` والـ labels النهائية، و**المفتاح العام لتوقيع الـ runner plane** بتاع السيرفر ورقم إصداره. الـ runner بيثبّته.
- الـ endpoint ده الوحيد في الـ runner plane اللي من غير هوية runner، فعليه throttle لكل IP بنفس آلية `"LoginThrottles"`، والرد في حالة التوكن الغلط أو المنتهي أو المستهلك واحد (`INVALID_REGISTRATION_TOKEN`).

### المصادقة على كل طلب
- **challenge من غير حالة:** `AuthService.Challenge` بيرجّع nonce عشوائي 16 byte ومعاه `RunnerId` ووقت انتهاء (60 ثانية)، والكل متوقّع بـ MAC بمفتاح مشتق من `DHOW_SECRET_KEY`. مفيش كتابة في قاعدة البيانات لطلب لسه مش متوثّق.
- **`AuthService.Authenticate`:** الـ runner بيوقّع بـ Ed25519 على `"dhow/runner-auth/v1" ‖ origin ‖ رقم مفتاح السيرفر ‖ الـ challenge`. السيرفر بيتحقق من الـ MAC والوقت والتوقيع وحالة الـ runner، وبعدين بيدّي **access token** شكله `dhow_rat_` و32 byte، عمره 15 دقيقة، ومتخزّن SHA-256 في `"RunnerSessions"`. الـ nonce بيتخزّن في نفس الصف بـ unique index، فالتوقيع ما يتعادش.
- **جلسة واحدة لكل runner:** أي `Authenticate` جديد بيلغي الجلسة اللي قبله. ولو الجلسات اتبدّلت أكتر من 3 مرات في 10 دقايق (علامة إن ملف الهوية اتنسخ على جهازين)، الـ runner بيتقفل أوتوماتيك (`Disabled`) وبيتسجّل حدث في الـ audit.
- **كل RPC تاني** بيبعت `Authorization: Bearer dhow_rat_...`. الفحص بيحصل في `WithRequestGate` بتاع connect-go، يعني قبل فك الضغط وقبل قراءة الـ protobuf. وكل طلب بيقرا صف الجلسة وحالة الـ runner من غير cache، زي جلسات المتصفح.
- **إثبات الامتلاك عند الحاجات الحساسة:** التوكن لوحده مايكفيش عشان يبدأ job ويستلم أسراره. `LeaseService.Start` لازم يكون معاه توقيع جديد بالمفتاح الخاص على `lease_id ‖ fencing_token ‖ digest الـ grant`. فلو توكن اتسرق من log أو proxy، مايقدرش يجيب أسرار.

### الـ runner بيتأكد من السيرفر إزاي
- **TLS إجباري:** الـ runner بيتحقق من شهادة السيرفر بـ roots النظام، أو بـ CA مخصوص (`--ca-file`). مفيش `insecure-skip-verify`. الاستثناء الوحيد `--dev` على `http://localhost` أو `127.0.0.1`.
- **توقيع على مستوى التطبيق:** السيرفر عنده مفتاح Ed25519 للـ runner plane، مشتق بـ HKDF من `DHOW_SECRET_KEY` (label `dhow/runner-plane-signing/v1`). كل `LeaseGrant` (فيه الـ job spec والـ fencing token) متوقّع بيه، والـ runner بيرفض أي grant توقيعه غلط أو `RunnerId` اللي فيه مش هو. يعني حتى لو الـ proxy أو الـ ingress اتخترق، مايقدرش يحقن job ينفّذ كود على أجهزة التشغيل.

### الإلغاء والتدوير
- **الإلغاء فوري:** الأدمن بيعمل revoke، فـ `"Runners"."State"` بيبقى `revoked` وكل الجلسات بتتلغي. أول RPC بعدها بيرجع `PERMISSION_DENIED`. والـ leases اللي معاه بتتعلّم `revoked`، والـ attempts بتبقى `lost` وماتترجعش لنفس الـ runner ([ADR-0047](0047-job-state-machine-and-reconciliation.md)). الإلغاء نهائي، والرجوع بتسجيل جديد.
- **التعطيل المؤقت:** `Disabled` بيمنع أي lease جديد ويسيب الشغّال يخلص.
- **تدوير مفتاح الـ runner:** المشغّل بيشغّل `sudo dhow-runner rotate-keys`. ده RPC `RotateKeys` متوقّع بالمفتاح القديم والجديد مع بعض، فـ `"KeyVersion"` بيزيد والجلسات بتتلغي. الواجهة و`doctor` بيحذّروا لو المفتاح عدّى عليه 365 يوم.
- **تدوير مفتاح السيرفر:** مربوط بـ `DHOW_SECRET_KEY`. في M3، تغيير `DHOW_SECRET_KEY` معناه إعادة تسجيل الـ runners، والوثائق بتقول كده صراحة. M5 (تدوير المفاتيح) هيضيف فترة تداخل بمفتاحين.

## البدائل اللي اتدرست
- **mTLS بشهادات من CA داخلي:** مربوط بالقناة وstandard، لكن:
  - الـ TLS بيتقفل عند الـ proxy في المسار الموثّق، فالشهادة مش هتوصل للسيرفر إلا بـ TLS passthrough أو port عام تاني، أو header من الـ proxy لازم نثق فيه.
  - الـ CA الداخلي بيبقى سر جديد من الدرجة الأولى: لو اتسرق، أي حد يطلّع شهادات runners.
  - الإلغاء محتاج CRL أو OCSP، أو شهادات قصيرة العمر، وده محتاج endpoint تجديد بمصادقة تانية أصلًا.
  - Cloudflare والـ ingress controllers بيصعّبوا client certs.
- **توكن bearer طويل العمر:** أبسط حاجة، لكنه بيتسرق من أي log أو proxy، ومش بيثبت امتلاك أي مفتاح.
- **توقيع كل طلب (HTTP message signatures):** أقوى ضد الـ replay، لكنه محتاج cache للـ nonces وتوقيع على الـ body. إثبات الامتلاك عند `Start` والتوكن القصير بيغطّوا الخطر الحقيقي بتعقيد أقل.
- **توكن ServiceAccount من Kubernetes أو SPIFFE/SPIRE:** مناسبين لـ Kubernetes بس، أو اعتمادية تشغيل تقيلة. ممكن نضيفهم بعدين كطريقة تسجيل إضافية فوق نفس الهوية.

## العواقب
- مفيش CA ولا شهادات نديرها. الـ TLS بتاع الـ proxy كفاية للقناة، والهوية في التطبيق.
- **جداول جديدة:** `"Runners"` و`"RunnerRegistrationTokens"` و`"RunnerSessions"`.
- **اختبارات لازمة:**
  - الإلغاء بيقطع أول RPC بعده.
  - توقيع `Authenticate` ما يتعادش.
  - grant متوقّع بمفتاح غلط بيترفض في الـ runner.
  - توكن تسجيل منتهي أو مستهلك أو ملغي بيرجّع نفس الخطأ.
  - runner من مؤسسة مايشوفش jobs مؤسسة تانية.
- تغيير `DHOW_SECRET_KEY` بيكسر تسجيل الـ runners لحد ما M5 تضيف التداخل.
- الـ labels اللي الـ runner بيبلّغ عنها نفسه (القدرات) بتستخدم للمطابقة بس، والحصر بالبيئات بالـ labels اللي الأدمن حددها.
