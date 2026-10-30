# ADR-0073: MPTCP مقفول في كل ملفات داو التنفيذية

- **الحالة:** مقبول (إصلاح عاجل لـ CVE-2026-98432)
- **التاريخ:** 2026-10-30
- **بيكمّل:** [ADR-0036](0036-interactive-host-installer.md) و[ADR-0066](0066-runner-install-and-encrypted-identity.md) (الـ `doctor`).

## السياق
- CVE-2026-98432: use-after-free بـ race في كود TCP لـ IPv4 في نواة Linux، CVSS 9.8، موجود من 4.4. التقرير المنشور بيقول إن أي حد من بره ومن غير حساب يقدر ياخد root على سيرفر عليه listener بـ multipath TCP (MPTCP).
- الإصلاح في النسخ الأصلية: 5.10.272 و5.15.223 و6.1.190 و6.6.159 و6.12.113 و6.18.56 و7.2.10 و7.3. والتوزيعات بتنقل الإصلاح لنسخها، فرقم النسخة لوحده مابيقولش النواة متصلّحة ولا لأ.
- من Go 1.24 الـ default بتاع GODEBUG `multipathtcp` بقى إن `net.Listen("tcp", ...)` بيفتح listener بـ MPTCP لو النواة مفعّلاه. إحنا بنبني بـ Go 1.27.3، ومفيش أي حاجة في الريبو كانت بتقفله. يعني listener الـ HTTP في السيرفر (وعليه الـ runner plane) وكل listener TCP في الـ runner (زي الـ egress proxy في الـ agent) كانوا MPTCP على أغلب التوزيعات.
- في Go 1.27 (`src/net/dial.go`): القيمة `0` بتقفل الـ listeners، و`3` بتقفلهم وتفتح الـ dialers، و`2` الـ listeners بس، و`1` الاتنين. والـ dialers مقفولين في الـ default أصلًا، ومابيتفتحوش غير بـ `1` أو `3`. يعني `0` بتقفل الاتنين.

## القرار
- **`godebug multipathtcp=0` في `go.mod` اللي في الـ root.** ده بيغطّي كل main package وكل test binary في الموديول: `dhow-server` و`dhow-runner` (والـ agent هو نفس الملف) و`dhow`. وبيتسجّل في الـ build info كـ `DefaultGODEBUG=multipathtcp=0`، و`go version -m` بيبيّنه.
- **كل طرق البناء جوه الموديول:** `make build*` و`deploy/release/dist.sh` والـ Dockerfile (بـ `-mod=readonly` وبينسخ `go.mod`) كلهم بيبنوا `./apps/.../cmd/...` من الـ root، فالسطر بيوصل لكلهم. مفيش `go build` لملف لوحده ولا موديول تاني.
- **الاختبارات في `internal/mptcp`:**
  - اختبار وقت التشغيل بيفتح listener بالطريقة العادية ويقرا `SO_PROTOCOL` من الـ socket، ويقارنه بـ listener متفتح بـ `SetMultipathTCP(true)`. لو التاني مش MPTCP، الجهاز مابيدعمش، فالاختبار بيتخطّى بدل ما يفشل. وكمان dialer عادي قدّام listener بـ MPTCP لازم يطلع TCP. (`TCPListener` مالوش `MultipathTCP()` في Go 1.27، فالفحص بـ `SO_PROTOCOL`.)
  - على الجهاز ده (6.18.44، `net.mptcp.enabled=1`) اختبار الـ listener بيفشل من غير السطر وبيعدّي بيه.
  - اختبار حارس بيقرا `go.mod`، وتاني بيقرا `DefaultGODEBUG` من الـ test binary نفسه، وتالت بيتأكد إن `ListenConfig{}` و`Dialer{}` بيقولوا `false`.
- **الـ `doctor` في السيرفر والـ runner** (من الـ engine المشترك، `internal/installer/kernel.go`): بيقرا `/proc/sys/net/mptcp/enabled` وإصدار النواة من `/proc/sys/kernel/osrelease` (نفس `uname -r`).
  - مفعّل: تحذير بيقول إن داو قافل MPTCP في الـ listeners بتاعته، وإن خدمات تانية على الجهاز ممكن لأ، وإن المشغّل لازم يركّب إصلاح النواة أو يحط `net.mptcp.enabled=0`، ورابط لـ `docs/security/kernel-advisories.md`.
  - مقفول، أو الملف مش موجود (MPTCP مش في النواة): سليم.
  - مش متقري: معلومة.
  - رقم النسخة بيتطبع بس، وعمره ما بيحدد النتيجة، بسبب الـ backports.
- **التحذير ده advisory:** الـ engine كان بيحسب أي تحذير من غير إصلاح «لسه فاضل» فالـ `doctor` يخرج بـ 1، وده كان هيخلّي `doctor` يفشل على أغلب الأجهزة حتى بعد ما النواة تتصلّح، لأن `net.mptcp.enabled=1` هو الـ default. فـ `Finding` بقى فيه `Advisory`: بيتطبع كـ تحذير مع النصيحة، بس `ApplyFixes` مابيعدّوش، والسطر الأخير بيبقى «Nothing blocks Dhow, but read the warnings above.». باقي التحذيرات زي ما هي.
- **الـ installers مابيغيّروش الـ sysctl.** ده إعداد بيأثّر على كل الخدمات اللي على الجهاز، فده قرار المشغّل. والـ `doctor` مالوش `Fix` هنا، فـ `--fix` مابيلمسوش.
- **الوثائق:** `docs/security/kernel-advisories.md` بالنسختين، ومتربوطة من وثائق التنصيب و`doctor` و`SECURITY.md`.

## البدائل اللي اترفضت
- **`SetMultipathTCP(false)` في كل listener:** محتاج نغيّر `http.Server.ListenAndServe` وكل `ListenConfig`، وأي listener جديد ممكن ينسى. سطر `go.mod` بيغطّي الكل مرة واحدة ومعاه اختبار.
- **`//go:debug multipathtcp=0` في كل `main`:** نفس النتيجة بس في كذا مكان، وأي binary جديد ممكن ينساه.
- **الـ installer يقفل `net.mptcp.enabled`:** بيكسر أي خدمة تانية بتستخدم MPTCP عن قصد، ومش من حق داو.
- **الحكم من رقم النسخة:** غلط مع الـ backports، فممكن يطمّن على نواة مش متصلّحة أو يحذّر من نواة متصلّحة.

## العواقب
- كل ملف تنفيذي بيستخدم TCP العادي في الاستماع والاتصال، حتى على نواة فيها MPTCP.
- **`GODEBUG` في الـ environment بيكسب على `go.mod`:** لو حد حط `GODEBUG=multipathtcp=1` أو `2` في بيئة الخدمة، MPTCP بيرجع. الوثيقة بتحذّر من ده. والـ agent جوه الـ sandbox مابيورثش `GODEBUG` أصلًا (ADR-0056).
- الحماية دي لداو بس. الخدمات التانية على نفس الجهاز لسه معرّضة لحد ما النواة تتصلّح، وعشان كده الـ `doctor` بيحذّر.
- أي ثغرة نواة تانية تأثّر على داو تتضاف في نفس الصفحة.
