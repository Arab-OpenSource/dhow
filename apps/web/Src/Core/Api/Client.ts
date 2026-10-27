import createClient, { type Middleware } from 'openapi-fetch';

import {
  API_ERROR_INVALID_RESPONSE,
  API_ERROR_NETWORK,
  ApiError,
  type TApiError,
} from '@/Core/Api/ApiError';
import type { paths } from '@/Core/Api/Schema';

export const API_BASE_PATH = '/api/v1';
const DEFAULT_LANGUAGE = 'ar';

const acceptLanguage: Middleware = {
  onRequest({ request }) {
    request.headers.set('Accept-Language', document.documentElement.lang || DEFAULT_LANGUAGE);
    return request;
  },
};

export const createApiClient = (fetchImpl: typeof fetch = (...args) => globalThis.fetch(...args)) => {
  const client = createClient<paths>({ baseUrl: `${window.location.origin}${API_BASE_PATH}`, fetch: fetchImpl });
  client.use(acceptLanguage);
  return client;
};

export const ApiClient = createApiClient();

interface IEnvelopeBody<TData> {
  Success: boolean;
  Data?: TData;
  Error?: TApiError;
}

export interface IFetchResult<TData> {
  data?: IEnvelopeBody<TData>;
  error?: unknown;
  response: Response;
}

export interface IUnwrapOptions {
  acceptFailureData?: boolean;
}

const isEnvelope = (value: unknown): value is IEnvelopeBody<unknown> =>
  typeof value === 'object' && value !== null && 'Success' in value;

const failure = (body: unknown, status: number): ApiError => {
  if (isEnvelope(body) && body.Error) return new ApiError(body.Error, status);
  return new ApiError({ Code: API_ERROR_INVALID_RESPONSE, Message: '' }, status);
};

export const unwrap = <TData>(result: IFetchResult<TData>, options: IUnwrapOptions = {}): TData => {
  const body: unknown = result.data ?? result.error;
  if (isEnvelope(body) && body.Data !== undefined && (body.Success || options.acceptFailureData)) {
    return body.Data as TData;
  }
  throw failure(body, result.response.status);
};

export const request = async <TData>(
  call: () => Promise<IFetchResult<TData>>,
  options?: IUnwrapOptions,
): Promise<TData> => {
  try {
    return unwrap(await call(), options);
  } catch (error) {
    if (error instanceof ApiError) throw error;
    const code = error instanceof SyntaxError ? API_ERROR_INVALID_RESPONSE : API_ERROR_NETWORK;
    throw new ApiError({ Code: code, Message: '' }, 0);
  }
};
