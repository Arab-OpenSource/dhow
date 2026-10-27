# ADR-0005: REST عام + OpenAPI؛ وConnect/gRPC للـ runners؛ وSSE للواجهة الحيّة

- **الحالة:** مقبول (مرحلة التصميم)
- **التاريخ:** 2026-10-27

## السياق
المستخدمين والـ CLI وTerraform والتكاملات محتاجين API سهل الاستخدام. والـ runners محتاجة streaming فعّال وعقود مكتوبة بأنواع (typed contracts).

## القرار
الـ API العام: REST ومواصفة OpenAPI 3.1 هي المصدر المرجعي. التواصل بين الـ runner والـ control plane: Connect/gRPC فوق HTTP/2. التحديثات الحيّة للواجهة: SSE موثَّق بالجلسة.

## البدائل اللي اتدرست
REST بس (streaming أضعف للـ runners)؛ Connect في كل حتة (أصعب على مستخدمي curl).

## العواقب
صيغتا عقد لازم نصونهم (OpenAPI + protobuf). فصل واضح بين الواجهة العامة والداخلية.
