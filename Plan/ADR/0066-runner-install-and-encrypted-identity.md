# ADR-0066: تغليف الـ runner، الجزء ب: الـ engine المشترك، و`dhow-runner install` و`doctor` و`uninstall`، والهوية المشفّرة بـ `systemd-creds`

- **الحالة:** مقبول (قرار مفوَّض في M3.10، الجزء ب)
- **التاريخ:** 2026-10-29
- **بيكمّل:** [ADR-0048](0048-runner-packaging-and-kubernetes.md) و[ADR-0065](0065-runner-packaging-part-a.md) و[ADR-0050](0050-runner-identity-implementation.md) و[ADR-0036](0036-interactive-host-installer.md).

## السياق
- ADR-0048 قال إن الـ engine بيتنقل لـ `internal/installer`، وإن التسجيل بيشفّر ملف الهوية بـ `systemd-creds` «لو متاح»، وإن `uninstall` بيلغي الـ runner «لو يقدر».
- التنفيذ طلّع أسئلة مالهاش إجابة هناك:
  - إزاي ننقل الـ engine من غير ما نلمس اختبارات السيرفر.
  - مين بيفك الهوية المشفّرة: الـ runner شغّال بمستخدم عادي، و`systemd-creds` بمفتاح الجهاز محتاج root.
  - إزاي `rotate-keys` يفضل آمن من الوقعة مع ملف مشفّر.
  - `doctor` يتأكد إن الـ runner بيدخل إزاي من غير ما يبدّل جلسة الخدمة الشغّالة.
  - الـ runner يلغي نفسه بإيه، والـ API مافيهوش طريق لده.

## القرار

### الـ engine في `internal/installer`
- **اتنقل:**
  - الخطوات والخطة والتنفيذ بالـ rollback.
  - طباعة الخطة والنتايج، ولفّة الإصلاح بتاعة `doctor`.
  - التوزيعة (`OSRelease` والعيلة) وخطوات الـ packages لـ apt وdnf.
  - قراءة ملف الإجابات (حقول معروفة بس، ووثيقة واحدة، و64 KiB).
  - الـ prompt بالإدخال المخفي، وملفات env.
  - آلية الرسايل: كتالوج للـ engine بيتدمج تحت كتالوج كل تطبيق.
- **فيه كمان `internal/installer/fakesys`:** نظام وهمي متصدّر، بيسجّل كل أمر والـ stdin بتاعه، عشان اختبارات الـ runner.
- **السيرفر:**
  - بيحتفظ بخططه وأسئلته ورسايله.
  - الأنواع عنده `type X = engine.X`، والدوال wrappers صغيرة.
  - النتيجة إن اختبارات `Core/Installer` اشتغلت من غير ولا تعديل، حتى في الـ imports.
  - الاستثناء الوحيد `Option` و`Prompter`: فضلوا أنواع محلية، لأن الاختبارات بتكتب `Option{{"a", "A"}}` من غير أسماء حقول، وده مسموح بس لنوع من نفس الـ package.
  - `Supported` في التوزيعة سياسة السيرفر، فبقى حقل في `Distro` بتاعه فوق `engine.OSRelease`.
- **`System` ماتغيّرش:** الـ fake بتاع السيرفر بيطبّقه زي ما هو. فأي حاجة الـ runner محتاجها زيادة (المالك بالرقم مثلًا) بتيجي من أوامر زي `getent`، مش من methods جديدة.
- **hook جديد:** `internal/` (الـ packages المشتركة) ممنوع يستورد `apps/`. والـ hook بقى يعرف طبقة `Installer` في الـ runner، وهي بتستورد `Config` و`Identity` بس، و`App` بس اللي بيستوردها.

### `dhow-runner install`
- **الخطة:**
  1. الفحص (Podman 5.4+ وcrun وnewuidmap وcgroup v2 والـ userns وإعداد AppArmor في Ubuntu)، وأي نقص بيوقف الخطة قبل ما تبدأ.
  2. تنصيب الناقص من التوزيعة بعد السؤال (`install_packages`)، وبعده فحص نسخة Podman تاني.
  3. المستخدم، و1048576 sub-id من أول نطاق فاضي من 1048576 وطالع.
  4. `/var/lib/dhow-runner` بـ `0700`، و`/etc/dhow-runner` بـ `0750`، و`runner.env` بـ `0640 root:dhow-runner`.
  5. الـ binary والوحدة (الوحدة متضمّنة في الـ binary، واختبار بيتأكد إنها نفس `deploy/systemd/dhow-runner.service`).
  6. الـ drop-in بتاع الهوية، والتسجيل، والإجابات من غير التوكن.
  7. التفعيل والتشغيل، وبعدهم انتظار سطر `the runner is ready` في سجل التشغيلة الحالية (`_SYSTEMD_INVOCATION_ID`)، بحد دقيقتين.
- **التوكن:** بيتقري من ملف أو stdin أو سؤال مخفي، وبيفضل في الذاكرة بس، ومابيروحش لأي أمر. الاختبارات بتدوّر عليه في argv والـ env والـ stdin بتاع كل أمر، وفي المخرجات وكل ملف اتكتب.
- **`--answers` معناه من غير أسئلة خالص:** حتى سؤال التأكيد الأخير. والتوكن ممكن ييجي من `token_file: "-"`، فالسؤال كان هيقرا من نفس الـ stdin.
- **التسجيل جوه نفس الـ process:** الـ installer بيبني الـ config من `runner.env` بخيار `Privileged`، لأن root بيشتغل على مجلد مملوك لـ `dhow-runner`. وبيستدعي نفس `RegisterWith` بتاع `register`، من خلال interface اسمه `Protocol` بيطبّقه `App`. والـ installer نفسه مابيلمسش أي مفتاح خاص.

### الهوية المشفّرة
- **التخزين ورا `identity.Store`:**
  - `FileStore` هو ملف الجزء أ، ومعاه مالك بيتعمل له `chown` لما root يكتب نيابة عن الـ runner.
  - `SealedStore` بيحفظ `identity.cred` و`identity.pending.cred` مشفّرين بـ `systemd-creds encrypt --name=identity`.
  - البيانات بتعدّي على الـ stdin والـ stdout، فمفيش نسخة صريحة على القرص في أي لحظة.
- **مين بيفك:**
  - **الخدمة:** drop-in فيه `LoadCredentialEncrypted=identity:/var/lib/dhow-runner/identity.cred`. فـ systemd بيفكه وقت البدء في `$CREDENTIALS_DIRECTORY` (ذاكرة، والخدمة بس تقراه)، و`run` بيقرا من هناك.
  - **root بس للباقي:** `rotate-keys` و`doctor` و`uninstall`.
- **ليه مش الملف الصريح ونعمله shred:** الـ journaling وCOW (btrfs) بيسيبوا نسخ، وشرط الخطة إن `grep` على القرص مايلاقيش المفتاح.
- **الكشف:** محاولة `systemd-creds encrypt` فعلية على بيانات تجربة، وبعدها `has-tpm2` يحدد نوع المفتاح. ده أدق من إن الأمر موجود وخلاص.
- **`identity: auto`** بيشفّر لو الجهاز يقدر. ولو مايقدرش، بيفضل ملف `0600` والخطة والنتيجة و`doctor` بيقولوا كده. والهوية الصريحة اللي سابها إعداد يدوي بتتشفّر في مكانها.

### `rotate-keys` مع الهوية المشفّرة
- **بيشتغل بـ root:** بيقرا `runner.env` بنفسه، والـ config بـ `Privileged`، والقفل لو root عمله بياخد مالك المجلد.
- **نفس الترتيب بالظبط:**
  1. `identity.pending.cred` بيتكتب قبل ما السيرفر يعرف.
  2. الطلب بالتوقيعين.
  3. بعد التأكيد `identity.cred` بيتكتب، وبعدين `pending` بيتمسح.
- **`run` مايقدرش يفك `identity.pending.cred`:**
  - لو المفتاح الحالي دخل، يبقى السيرفر ماخدش المفاتيح الجديدة، فالملف بيتمسح (هو مشفّر، فمسحه بس كفاية).
  - لو اترفض، بيقف برسالة بتقول شغّل `sudo dhow-runner rotate-keys`. والأمر ده بيكمّل التدوير بنفس قرار الجزء أ: المحفوظ بيتستخدم بس لو السيرفر شايله.
  - ده نفس الأمان بالظبط، بس الإكمال التلقائي في `run` بقى خطوة يدوية واحدة.
- **كل نقط الوقعة ليها اختبار** بـ sealer وهمي (AES-GCM)، وبعده بدء الخدمة أو تدوير تاني، وبعدهم job بسر. والاختبار بيتأكد إن المفاتيح (خام وbase64 وhex) مش موجودة في أي ملف.

### `doctor` وتسجيل الدخول
- **وهو الخدمة شغّالة، `doctor` مابيعملش `Authenticate`:** كان هيبدّل جلسة الخدمة، والخدمة هتبدّل تاني، و`doctor` مرتين في عشر دقايق كانوا هيقفلوا الـ runner كنسخة (ADR-0050).
- **بدل كده:**
  - `Challenge` بس، وده مابيقراش قاعدة البيانات ولا بيفتح جلسة، فبيثبت إن السيرفر بيرد.
  - وتسجيل الدخول بيتقري من سجل التشغيلة الحالية.
- **لو الخدمة واقفة**، `doctor` بيسجّل الدخول بنفسه.
- **`doctor` بيقيس كمان:** `systemd-analyze security` (الحد 6.9)، وملف `pending` اللي فضل، والصلاحيات والمالكين، ومكان الهوية.

### الـ runner بيلغي نفسه: `AuthService.Revoke`
- **مفيش طريق في الـ runner plane للإلغاء،** والـ REST محتاج جلسة مستخدم.
- **procedure جديدة `Revoke`:**
  - بسياسة `RunnerSession`.
  - فيها توقيع بالمفتاح الحالي على رسالة label جديد `dhow/runner-revoke/v1` (الـ origin ومعرّف الـ runner ورقم المفتاح)، فالـ access token لوحده مايكفيش.
- **السيرفر بيعمل نفس transaction بتاعة إلغاء اللوحة:** الحالة `revoked`، والجلسات بتتلغي، و`runner.revoked` في الـ audit من غير مستخدم وبـ `Reason: revoked_by_runner`.
- **لو الـ runner مايقدرش يدخل** (السيرفر مش متاح، أو اتلغى قبل كده)، `uninstall` بيطبع معرّفه ويقول للمشغّل يلغيه من اللوحة، ويكمّل التنضيف.
- **`uninstall` بيحتفظ بالمستخدم والـ sub-ids** إلا مع `--remove-user`. ولو اتشغّل تاني بيقول مفيش حاجة يشيلها.

### الاختبارات والـ VM
- **`deploy/systemd/smoke-install.sh`:**
  - سيرفر حقيقي، و`install --answers`، و`online`.
  - `grep` على المفاتيح في القرص والـ journal، و`doctor`، وjob.
  - `rotate-keys` والهوية فاضلة مشفّرة، و`install` تاني من غير تغيير.
  - و`uninstall` والـ runner `revoked`.
- **فين بيشتغل:**
  - في container بـ systemd بالـ driver الوهمي (`make runner-install-e2e`).
  - وبـ Podman حقيقي في job الـ CI `runner-install`.
- **الـ job على `ubuntu-26.04` مش `ubuntu-24.04`:** Ubuntu 24.04 فيه Podman 4.9.3، أقل من حد ADR-0043. ونسخة الـ container شغّالة على `ubuntu-24.04`.
- **`deploy/test-vm/run.sh`:**
  - cloud image متحققة بالـ checksum تحت QEMU/KVM، وseed لـ cloud-init فيه الـ binaries.
  - Postgres 18 في container، ونفس الـ smoke بـ Podman حقيقي.
  - workflow أسبوعي وبالطلب، لأن الـ runners العادية في GitHub فيها `/dev/kvm`.

## البدائل اللي اتدرست
- **التسجيل كأمر فرعي بمستخدم `dhow-runner`** (`systemd-run --uid`): كان هيكتب الهوية صريحة على القرص قبل التشفير.
- **`systemd-creds --user`** عشان الـ runner يشفّر بنفسه: محتاج systemd 256+، وUbuntu 24.04 فيه 255.
- **`ImportCredential=` بدل `LoadCredentialEncrypted=`:** بيقبل الملف الناقص، لكنه بيقرا من مجلدات الـ credstore بس (زي `/etc/credstore.encrypted`)، والخطة طلبت `LoadCredentialEncrypted=`.
- **`doctor` يسجّل الدخول دايمًا:** بيبدّل جلسة الخدمة، وبيقرّب الـ runner من قفل النسخ.
- **`uninstall` يطلب API token:** خطوة زيادة على المشغّل لحاجة الـ runner يقدر يثبتها بمفتاحه.
- **نقل الـ engine بتغيير اختبارات السيرفر:** الـ aliases خلّت النقل من غير ما نلمسها.

## العواقب
- `runnersig` فيه label رابع (`revoke`)، و`AuthService` فيه 4 procedures (12 في الـ runner plane كله).
- **الهوية المشفّرة مابتتنقلش لجهاز تاني.** ده المقصود، والاسترجاع من backup على جهاز جديد محتاج تسجيل جديد.
- **`rotate-keys` بقى بـ root في الحالتين،** و`systemd-run --uid` لسه شغّال للهوية الصريحة.
- **الـ VM tests مااتشغّلتش على جهاز التطوير لأن مفيش KVM.** أول تشغيل ليها في الـ workflow الأسبوعي.
