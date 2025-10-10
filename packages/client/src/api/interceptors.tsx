import { ConnectError, createContextKey, Interceptor } from '@connectrpc/connect';

export const kErrorHandler = createContextKey<(error: ConnectError) => void>(() => void {});

const errorHandler: Interceptor = (next) => async (request) => {
  try {
    const response = await next(request);
    return response;
  } catch (error) {
    if (error instanceof ConnectError) request.contextValues.get(kErrorHandler)(error);
    throw error;
  }
};

export const defaultInterceptors = [errorHandler];
