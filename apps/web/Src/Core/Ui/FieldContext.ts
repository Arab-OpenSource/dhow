import { createContext } from 'react';

export interface IFieldContext {
  id: string;
  errorId: string | undefined;
  invalid: boolean;
}

export const FieldContext = createContext<IFieldContext | null>(null);
