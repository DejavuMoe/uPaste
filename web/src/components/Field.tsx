import React, { useId } from 'react';

export interface FieldProps {
  label: string;
  htmlFor?: string;
  helperText?: string;
  error?: string;
  required?: boolean;
  className?: string;
  children: React.ReactNode;
}

export const Field: React.FC<FieldProps> = ({
  label,
  htmlFor,
  helperText,
  error,
  required,
  className = '',
  children,
}) => {
  const generatedId = useId();
  const fieldId = htmlFor || generatedId;
  const helperId = helperText ? `${fieldId}-helper` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;

  const describedBy = [errorId, helperId].filter(Boolean).join(' ') || undefined;

  // Clone single element child if appropriate, or render children with context/props
  const renderedChildren = React.Children.map(children, (child) => {
    if (React.isValidElement(child)) {
      return React.cloneElement(child as React.ReactElement<any>, {
        id: (child.props as any).id || fieldId,
        'aria-describedby': (child.props as any)['aria-describedby'] || describedBy,
        'aria-invalid': error ? true : (child.props as any)['aria-invalid'],
      });
    }
    return child;
  });

  return (
    <div className={`form-field ${error ? 'has-error' : ''} ${className}`.trim()}>
      <label htmlFor={fieldId} className="form-label">
        {label}
        {required && <span className="form-required" aria-hidden="true"> *</span>}
      </label>
      {renderedChildren}
      {error && (
        <div id={errorId} className="form-error" role="alert">
          {error}
        </div>
      )}
      {!error && helperText && (
        <div id={helperId} className="form-helper">
          {helperText}
        </div>
      )}
    </div>
  );
};
