export type TApiError = Readonly<{ Code: string; Message: string }>;

export const API_ERROR_NETWORK = 'NETWORK';
export const API_ERROR_INVALID_RESPONSE = 'INVALID_RESPONSE';

export class ApiError extends Error implements TApiError {
  readonly Code: string;
  readonly Message: string;
  readonly Status: number;

  constructor(error: TApiError, status: number) {
    super(error.Message);
    this.name = 'ApiError';
    this.Code = error.Code;
    this.Message = error.Message;
    this.Status = status;
  }
}

export const isApiError = (value: unknown): value is ApiError => value instanceof ApiError;
