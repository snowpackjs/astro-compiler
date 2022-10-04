// eslint-disable-next-line no-shadow
export const enum DiagnosticCode {
  ERROR = 1000,
  ERROR_UNTERMINATED_JS_COMMENT = 1001,
  ERROR_FRAGMENT_SHORTHAND_ATTRS = 1002,
  ERROR_UNMATCHED_IMPORT = 1003,
  ERROR_UNSUPPORTED_SLOT_ATTRIBUTE = 1004,
  WARNING = 2000,
  WARNING_UNTERMINATED_HTML_COMMENT = 2001,
  WARNING_UNCLOSED_HTML_TAG = 2002,
  WARNING_DEPRECATED_DIRECTIVE = 2003,
  WARNING_IGNORED_DIRECTIVE = 2004,
  WARNING_UNSUPPORTED_EXPRESSION = 2005,
  WARNING_SET_WITH_CHILDREN = 2006,
  INFO = 3000,
  HINT = 4000,
}
