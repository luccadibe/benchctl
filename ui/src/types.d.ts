declare module "papaparse" {
  type ParseMeta = {
    fields?: string[];
  };

  type ParseResult<T> = {
    data: T[];
    meta: ParseMeta;
  };

  type ParseConfig = {
    header?: boolean;
    skipEmptyLines?: boolean;
  };

  function parse<T>(input: string, config?: ParseConfig): ParseResult<T>;

  const Papa: {
    parse: typeof parse;
  };

  export default Papa;
}

declare module "*.module.css" {
  const classes: Record<string, string>;
  export default classes;
}

declare module "*.css";
