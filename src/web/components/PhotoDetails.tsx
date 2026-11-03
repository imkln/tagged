import { Fragment } from 'react';
import type { Photo } from '../hooks/useLibrary';

export default function PhotoDetails({ photo }: { photo: Photo }) {
  const camera = [photo.cameraMake, photo.cameraModel]
    .map((value) => value?.trim())
    .filter(Boolean)
    .join(' ');
  const hasDimensions =
    Number.isInteger(photo.width) &&
    Number.isInteger(photo.height) &&
    photo.width! > 0 &&
    photo.height! > 0;
  const hasCoordinates =
    typeof photo.latitude === 'number' &&
    typeof photo.longitude === 'number' &&
    Number.isFinite(photo.latitude) &&
    Number.isFinite(photo.longitude) &&
    Math.abs(photo.latitude) <= 90 &&
    Math.abs(photo.longitude) <= 180;

  const fields = [
    { label: 'Filename', value: photo.filename },
    { label: 'Format', value: 'JPEG' },
    { label: 'Capture date', value: displayDate(photo.capturedAt) },
    { label: 'Upload date', value: displayDate(photo.uploadedAt) },
    {
      label: 'Dimensions',
      value: hasDimensions ? `${photo.width} × ${photo.height}` : undefined,
    },
    { label: 'Camera', value: camera },
    {
      label: 'GPS',
      value: hasCoordinates
        ? `${photo.latitude}, ${photo.longitude}`
        : undefined,
    },
  ];

  return (
    <dl>
      {fields
        .filter(({ value }) => value)
        .map(({ label, value }) => (
          <Fragment key={label}>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </Fragment>
        ))}
    </dl>
  );
}

function displayDate(value?: string) {
  if (!value || Number.isNaN(Date.parse(value))) return undefined;

  // Preserve the camera's recorded time and offset, including no offset.
  return value.replace('T', ' ').replace(/Z$/, ' UTC');
}
