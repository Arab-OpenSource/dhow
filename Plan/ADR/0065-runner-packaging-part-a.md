# ADR-0065: تغليف الـ runner، الجزء أ: `rotate-keys` والرجوع من أي وقعة، ووحدة systemd والتقوية اللي اتقبلت، والأرشيفات و`get.sh --component runner`

- **الحالة:** مقبول (قرار مفوَّض في M3.10، الجزء أ)
- **التاريخ:** 2026-10-29
- **بيكمّل:** [ADR-0048](0048-runner-packaging-and-kubernetes.md) و[ADR-0050](0050-runner-identity-implementation.md) و[ADR-0053](0053-runner-implementation.md).

## السياق
- ADR-0048 حدّد الـ binary والوحدة و`rotate-keys` و`get.sh --component runner`، وساب «التقوية الكاملة بتتحدد بالتجربة».
- ناحية السيرفر من `RotateKeys` موجودة من M3.2: رسالة واحدة موقّعة بالمفتاح القديم والجديد، و`"KeyVersion"` بيزيد، والجلسات بتتلغي، كله في transaction واحدة.
- الأسئلة اللي فضلت: الـ runner يعرف رقم مفتاحه منين، وإيه اللي يحصل لو وقع بين قبول السيرفر وتبديل الملف، وإمتى يتسمح بالتدوير، وأنهي directives تنفع مع Podman rootless.

## القرار

### ملف الهوية
- **الإصدار 2:** فيه `KeyVersion`، لأن رسالة `RotateKeys` بتتوقّع على الرقم الحالي. التسجيل بيكتب 1، والملف بإصدار 1 بيتقري على إنه 1 (مفيش runner قبل كده كان يقدر يدوّر).
- **التخزين:** نفس قواعد `register` بالظبط: ملف `0600` بيتكتب atomic، ومن غير `systemd-creds` لأن `register` مابيستخدموش لسه. ده بييجي مع `install` في الجزء ب.

### `rotate-keys`
- **الـ runner لازم يكون واقف:**
  - `run` بيمسك قفل `flock` على `lock` في مجلد الحالة طول ما هو شغّال، و`rotate-keys` بيرفض لو القفل مش متاح. ومحاولة `run` تانية على نفس المجلد بترجع خطأ برضه.
  - و`rotate-keys` بيرفض كمان لو الـ journal فيه attempts لسه ماتبلّغتش.
  - مفيش `--while-running`، لأن الهوية بيستخدمها الـ executor في توقيع `Start` وفك الأسرار، وتبديلها وسط الشغل محتاج تنسيق ماله لازمة.
- **الترتيب:**
  1. الهوية الجديدة كاملة بتتكتب في `identity.pending` (ملف مؤقت، وfsync، وrename، وfsync للمجلد) **قبل** ما السيرفر يعرف بيها.
  2. `RotateKeys` بتوقيع المفتاحين.
  3. بعد التأكيد، الهوية الجديدة بتتكتب فوق `identity` بنفس الطريقة الـ atomic، وبعدين `identity.pending` بيتمسح. والهوية دي بتتبني من الهوية الحالية، والمفاتيح بس اللي بتيجي من `identity.pending`، فأي حقل تاني في الملف ده مابيوصلش للهوية.
- **المفاتيح المحفوظة بتتستخدم بس لو السيرفر شايلها فعلًا.** `rotate-keys` و`run` الاتنين بيقرروا بنفس الطريقة لما يلاقوا `identity.pending`:
  - بيجرّبوا المفتاح الحالي الأول. لو دخل، يبقى السيرفر ماخدش المفاتيح المحفوظة، لأن التبديل بيحصل بس من رقم المفتاح اللي التوقيعين عليه (compare-and-swap على `KeyVersion`). فالملف بيتكتب فوقه أصفار وبيتمسح، و`rotate-keys` بيعمل مفاتيح جديدة، و`run` بيكمّل بالحالي ويكتب في الـ log إنه مسحه.
  - لو الحالي اترفض والمحفوظ دخل، بيكتبوا الهوية الجديدة ويكمّلوا بيها.
  - لو الاتنين اترفضوا، `rotate-keys` بيقف بخطأ، و`run` بيكمّل بالحالي وبالـ backoff العادي.
  - والسبب: مفاتيح فضلت على القرص من غير تأكيد ممكن تكون اتنسخت (backup أو snapshot). فمابتتبعتش للسيرفر أبدًا.
  - و`run` بيسلّم الـ client اللي دخل للـ runner، فمفيش تبديل جلسة زيادة يتحسب في كشف النسخ.
- **نقط الوقوع:** قبل كتابة `pending` أو أثناءها، وبعدها وقبل الطلب، وبعد قبول السيرفر وقبل الرد، وبعد الرد وقبل كتابة الهوية، وبعد كتابتها وقبل مسح `pending`. كلها ليها اختبار بيحقن فشل في المكان ده، وبعده إقلاع أو تدوير تاني، وبعدهم job بسر بيشتغل بالهوية اللي على القرص.
- **الملفات المؤقتة** اللي فضلت من كتابة اتقطعت (`.identity*.tmp-*`) بتتمسح في أول `run` أو `rotate-keys`، والاتنين ماسكين القفل.

### الوحدة `deploy/systemd/dhow-runner.service`
- **من ADR-0048 كما هي:**
  - `User=dhow-runner` و`Delegate=yes` و`MemorySwapMax=0` و`LimitCORE=0` و`KillMode=control-group`.
  - `ProtectSystem=strict` مع `ReadWritePaths=/var/lib/dhow-runner`، و`ProtectHome` و`PrivateTmp`.
  - `ProtectKernelModules` و`ProtectClock` و`RestrictRealtime` و`LockPersonality`.
  - `Restart=on-failure` و`EnvironmentFile=/etc/dhow-runner/runner.env`.
- **زيادة، واتجرّبت كلها مع Podman rootless جوه الوحدة:**
  - `CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_DAC_OVERRIDE CAP_SYS_ADMIN CAP_SYS_PTRACE`. الـ runner نفسه مالوش capabilities، والـ bounding set بيحدّد بس اللي يقدر ياخده برنامج setuid. والخمسة دول هما اللي `newuidmap` الـ setuid بتاع Debian وUbuntu محتاجهم، و`CAP_SYS_PTRACE` بالذات عشان يشوف الـ process الجديد لما `/proc` مخبّي باقي المستخدمين. والـ user namespace بتاع الـ job بيبدأ بمجموعة جديدة كاملة، فالـ jobs ماتتأثرش.
  - `ProtectProc=invisible`: الـ runner بيشوف processes الـ jobs بتاعته بس، لأنه مالك الـ user namespaces بتاعتها.
  - `AmbientCapabilities=` و`PrivateIPC` و`RemoveIPC` و`KeyringMode=private`.
  - `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK`: نفس العائلات اللي الـ sandbox بيسمح بيها.
  - `SystemCallFilter=~@clock @module @obsolete @raw-io @swap` و`SystemCallErrorNumber=EPERM`. الـ filter ده بيتورّث للـ jobs، فاتختار بحيث مايقفلش أي حاجة الـ job يقدر يعملها النهارده: كل اللي فيه محتاج capabilities الـ host أو قديم.
- **اللي اتساب وسببه:**
  - `NoNewPrivileges` و`PrivateUsers`: بيكسروا `newuidmap`.
  - `RestrictNamespaces`: Podman محتاج كل الأنواع.
  - `ProtectKernelTunables` و`ProtectKernelLogs` و`ProtectHostname` و`ProcSubset`: بيغطّوا أجزاء من `/proc`، فالكيرنل بيرفض mount الـ `/proc` الجديد جوه الـ job.
  - `ProtectControlGroups`: الـ runner بيكتب حدود الـ job في الـ subtree بتاعه.
  - `PrivateDevices` و`DevicePolicy=closed`: الـ container مابيقومش.
  - `MemoryDenyWriteExecute` و`SystemCallArchitectures=native`: بيتورّثوا، فبيكسروا الـ JIT وbinaries الـ 32-bit اللي الـ sandbox بيسمح بيها.
  - `RestrictSUIDSGID`: Podman مابيقدرش يفك image فيها ملف setuid.
  - `@system-service` ومجموعات `@mount` و`@privileged` و`@debug` و`@cpu-emulation` و`@reboot` و`@resources` و`@keyring`: Podman محتاجها، أو بتقفل حاجات الـ jobs مسموح لها بيها.
  - `UMask=0077`: فايدته قليلة، ومااتجرّبش مع الـ driver كله.
  - `PrivateNetwork` و`IPAddressDeny`: الـ runner محتاج الشبكة.
- **التقييم:** `systemd-analyze security` بيدّي 5.2 MEDIUM. الـ CI بيفشل فوق 6.9، والرقم متسجّل في `docs/runner/install.md`.
- **التحقق في الـ CI (job `isolation` على `ubuntu-26.04`):**
  - الـ escape والـ limits suites بيشتغلوا تاني جوه نفس الـ sandbox بالظبط. والـ properties بتتقري من ملف الوحدة نفسه (`deploy/systemd/sandbox-properties.sh`)، فمفيش نسخة تانية تقدر تختلف عنه.
  - وsmoke بيشغّل السيرفر وrunner حقيقي تحت الوحدة المشحونة، ويستنى `online`، ويوقف ويدوّر المفاتيح، ويستنى `online` تاني.

### الأرشيفات و`get.sh`
- **`dist.sh`:** بيبني `dhow-runner` لـ `amd64` و`arm64` بنفس الـ flags والـ tar الثابت بتاع السيرفر، في نفس `SHA256SUMS`. و`DIST_COMPONENTS` بيسمح ببناء الـ runner لوحده للاختبار.
- **`get.sh --component runner`:**
  - نفس التحقق بالظبط: cosign على `SHA256SUMS`، وبعدين السطر بتاع الأرشيف بالاسم الكامل، ولو السطر مش موجود بيرفض.
  - بيركّب الـ binary والوحدة، ومابيشغّلش `dhow-runner install`، وبيطبع الخطوات اليدوية.
  - و`DHOW_DOWNLOAD_URL` بيغيّر مكان التنزيل بس (`https://` بس)، والتوقيع لسه لازم يكون من workflow الريبو.
  - **cosign:** لو `DHOW_DOWNLOAD_URL` أو `DHOW_REPO` متغيّرين، cosign إجباري، ومن غيره `get.sh` بيقف. لأن الـ mirror هو اللي بيقدّم الأرشيف و`SHA256SUMS` مع بعض، فالـ checksum لوحده مابيثبتش أي حاجة.
  - والخروج الوحيد `--insecure-skip-signature`، بتحذير واضح، وبيستخدمه اختبار الـ fixture المحلي بس.
  - من GitHub نفسه (الافتراضي) من غير cosign بيفضل تحذير بس للسيرفر والـ runner زي ما كان. والسبب إن الأرشيف و`SHA256SUMS` جايين من نفس الـ release على GitHub بـ TLS، فالـ checksum بيكشف التلف، ومن يقدر يغيّر الاتنين هناك يقدر يغيّر `get.sh` نفسه. وإجبار cosign هنا كان هيكسر التنصيب على أجهزة كتير من غير مكسب حقيقي.
  - **سباق نفس المستخدم:** لما الـ script بيشتغل من غير root، الملفات بتتنسخ لمجلد بيعمله `sudo mktemp -d`. والتوقيع والـ checksum والفك والتركيب كلهم بيحصلوا من النسخة دي، فـ process تاني بنفس المستخدم مايقدرش يبدّل الأرشيف بعد الفحص. والمجلد بيتمسح في الآخر.
- **الـ release:** بعد التوقيع، الـ workflow بيركّب الأرشيف الموقّع بـ `get.sh` نفسه من نسخة محلية، ومعاه cosign، وبيتأكد إن `SHA256SUMS` متلعوب فيه بيترفض. يعني سلسلة التحقق كلها بتتجرّب قبل النشر.

## البدائل اللي اتدرست
- **الـ runner يسأل السيرفر عن رقم مفتاحه:** محتاج حقل جديد في البروتوكول. والرقم بيتغيّر بس من الـ runner نفسه، فتخزينه أبسط.
- **توليد المفاتيح وإرسالها قبل حفظها:** لو الـ runner وقع بعد قبول السيرفر، المفتاح الجديد يضيع والقديم بيترفض، والـ runner لازم يتسجّل من الأول.
- **إعادة إرسال المفاتيح المحفوظة لو المفتاح الحالي لسه شغّال:** كانت النسخة الأولى من القرار، لكن المراجعة الأمنية لقت إن المفاتيح دي ممكن تكون اتسرّبت وهي مستنية. ومسحها آمن، لأن قبول المفتاح الحالي معناه إن السيرفر ماخدهاش.
- **مسح `identity.pending` على أي رفض من غير ما نجرّب:** رد زي `UNAUTHENTICATED` ممكن يكون معناه إن طلب قديم اتقبل. فالقرار بيتاخد بتجربة الدخول بالمفتاحين، مش بنوع الخطأ.
- **التدوير والـ runner شغّال:** محتاج يبدّل الهوية في الـ executor وفي الـ client مع بعض وسط leases شغّالة. والإيقاف دقيقة واحدة في السنة.
- **وحدة من غير bounding set:** تقييم أسوأ، وأي برنامج setuid الـ runner يشغّله ياخد كل الـ capabilities.

## العواقب
- ملف الهوية بإصدار 2 مابيتقريش بنسخة أقدم من الـ runner. والرجوع لنسخة أقدم محتاج تسجيل جديد.
- `ProtectProc=invisible` والـ bounding set اتجرّبوا على Fedora (systemd 259) بس، و`ubuntu-26.04` بيتجرّب في الـ CI. أي توزيعة `newuidmap` فيها محتاج capability زيادة هتفشل في فحوص الإقلاع، والحل drop-in بيوسّع الـ bounding set.
- الجزء ب (`install` و`doctor` و`uninstall` والـ engine المشترك والـ VM) بيبني على الوحدة دي وعلى `get.sh`، و`get.sh` هيشغّل `dhow-runner install` بدل ما يطبع الخطوات.
