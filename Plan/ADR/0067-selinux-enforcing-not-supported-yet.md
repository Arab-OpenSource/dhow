# ADR-0067: SELinux في وضع enforcing مش مدعوم دلوقتي

- **الحالة:** مقبول (قرار الـ maintainer في M3.10، الجزء ب)
- **التاريخ:** 2026-10-30
- **بيكمّل:** [ADR-0043](0043-job-sandbox-rootless-podman.md) و[ADR-0066](0066-runner-install-and-encrypted-identity.md).

## السياق
- `make vm-test` على Fedora 43 وRocky 10 (SELinux enforcing) طلّع إن كل job بيفشل بـ «the agent did not connect to the control channel».
- الـ agent جوه الحاوية بيتصل بالـ control socket اللي الـ runner بيسمع عليه في مجلد الـ attempt تحت `/var/lib/dhow-runner`.
- إضافة `relabel=private` على الـ mount بتاع مجلد التحكم (`Sandbox/Podman/Podman.Create.go`) صلّحت الوصول للملف نفسه، بس مش للاتصال.

## الدليل
اتنين AVC denials، واحد لكل domain جرّبناه للـ runner:
1. الـ runner كـ `unconfined_service_t` (الوحدة العادية): `avc: denied { connectto }` بـ `scontext` هو `container_t` (الـ agent) و`tcontext` هو `unconfined_service_t`، و`tclass=unix_stream_socket`.
2. الـ runner كـ `container_runtime_t`: نفس الـ denial بالظبط، و`tcontext` هو `container_runtime_t`.

السطور زي ما طلعت من `ausearch` على Rocky 10 (تشغيلات `vm-test` يوم 2026-10-30)، والمسار مختصر:

```
avc:  denied  { connectto } for  comm="agent" path=".../control/control.sock" scontext=system_u:system_r:container_t:s0:c674,c947 tcontext=system_u:system_r:unconfined_service_t:s0 tclass=unix_stream_socket permissive=0
avc:  denied  { connectto } for  comm="agent" path=".../control/control.sock" scontext=system_u:system_r:container_t:s0:c561,c760 tcontext=system_u:system_r:container_runtime_t:s0 tclass=unix_stream_socket permissive=0
```

يعني `connectto` بيتحسب على الـ domain بتاع الـ process اللي سامع على الـ socket، مش على label الملف، فمفيش relabel للملف يحلّها. والـ policy بتاعة `container-selinux` مابتسمحش لـ `container_t` يعمل `connectto` على أي domain منهم.

## القرار
- **SELinux enforcing مش مدعوم لحد ما نرجع للقرار ده.**
- **`dhow-runner install`:** قبل الخطوة 1 بيقرا `/sys/fs/selinux/enforce` من خلال الـ `System` بتاع الـ engine. لو فيه `1`، الخطة بتترفض من غير ما حاجة تتغيّر، والرسالة بالإنجليزي والعربي المبسّط (ADR-0062) بتقول إن الـ jobs مابتشتغلش مع SELinux enforcing لسه، وفيها رابط لـ «Supported hosts» في `docs/runner/install.md`، وإن `setenforce 0` قرار المشغّل وإحنا مش بننصح بيه.
- **`doctor`** بيطلّعها مشكلة بنفس الرسالة.
- **permissive وdisabled مقبولين.**
- **`relabel=private` فاضل على mount التحكم:** صح لوحده، وأي حل جاي محتاجه.
- **الـ VM test** (`.github/workflows/vm-test.yml`) بقى على `debian-13` بس. و`deploy/test-vm/run.sh` لسه بيعرف fedora وrocky-10 عشان نرجع لهم.
- **الوثائق:** Debian وUbuntu (AppArmor) مدعومين، وFedora وRHEL وRocky وAlma مع enforcing لسه لأ.

## البدائل اللي اترفضت دلوقتي
- **نقلب اتجاه قناة التحكم:** الـ agent يسمع على socket والـ runner هو اللي يتصل. ساعتها الـ `connectto` يبقى من domain الـ runner لـ `container_t`، وده غالبًا مسموح. اترفض دلوقتي لأنه بيغيّر بروتوكول الـ agent (ADR-0043 وADR-0056): الـ agent بيبقى سامع جوه الحاوية قبل ما يتأكد من مين بيكلّمه، وده محتاج مراجعة أمنية وإعادة كتابة جزء كبير من الـ Sandbox.
- **policy module خاص بـ domain للـ runner:** نعمل `dhow_runner_t` ونسمح لـ `container_t` يعمل `connectto` عليه بس. اترفض دلوقتي لأن ده module لازم يتبني ويتوقّع ويتنصّب ويتصان مع كل نسخة من `container-selinux` في كل توزيعة، ومحتاج اختبارات على كل واحدة.

## العواقب
- المشغّل على Fedora أو RHEL أو Rocky أو Alma بـ enforcing بيعرف من أول `install`، قبل ما الجهاز يتغيّر.
- مفيش اختبار VM على توزيعة SELinux في الـ CI لحد ما القرار يتغيّر.
- **القرار بيترجعله قبل ما M3 يقفل** (بند مفتوح في `Plan/Status.md`).
