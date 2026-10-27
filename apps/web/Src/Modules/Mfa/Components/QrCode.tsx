import { useEffect, useState } from 'react';

import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';

export interface IQrCodeProps {
  value: string;
}

interface IQrShape {
  Size: number;
  Path: string;
}

const QUIET_ZONE = 4;

const draw = async (value: string): Promise<IQrShape> => {
  const [{ generate, correction }, { toSvgPath }] = await Promise.all([import('lean-qr/nano'), import('lean-qr/extras/svg')]);
  const code = generate(value, { minCorrectionLevel: correction.M });
  return { Size: code.size, Path: toSvgPath(code) };
};

export const QrCode = ({ value }: IQrCodeProps) => {
  const t = useT();
  const [Shape, setShape] = useState<IQrShape | null>(null);

  useEffect(() => {
    let live = true;
    void draw(value).then((shape) => {
      if (live) setShape(shape);
    });
    return () => {
      live = false;
    };
  }, [value]);

  if (Shape === null) return <Skeleton label={t('totp.loading')} lines={3} />;
  const box = Shape.Size + QUIET_ZONE * 2;

  return (
    <svg
      role="img"
      aria-label={t('totp.qr')}
      viewBox={`${-QUIET_ZONE} ${-QUIET_ZONE} ${box} ${box}`}
      shapeRendering="crispEdges"
      className="mx-auto block size-48 rounded-md bg-primary fill-input"
    >
      <path d={Shape.Path} />
    </svg>
  );
};
