/**
 * Snippet Definitions — Raw Swaggo comment templates.
 *
 * Provides the empty/placeholder template using VS Code SnippetString
 * tab stop syntax for interactive field navigation.
 *
 * Covers ALL Declarative Comments Format from swaggo/swag:
 * https://github.com/swaggo/swag#api-operation
 */

/**
 * Generate a comprehensive Swaggo template with all annotation types as tab stops.
 * Used when the user types /genswagger:empty.
 */
export function emptyTemplate(): string {
  return [
    '// ${1:handlerName} godoc',
    '//',
    '//\t@Summary\t\t${2:Summary}',
    '//\t@Description\t${3:Description}',
    '//\t@ID\t\t\t\t${4:operationId}',
    '//\t@Tags\t\t\t${5:tag}',
    '//\t@Accept\t\t\t${6|json,xml,plain,html,mpfd,x-www-form-urlencoded,json\\,xml|}',
    '//\t@Produce\t\t${7|json,xml,plain,html|}',
    '//\t@Param\t\t\t${8:name}\t${9|query,path,body,header,formData|}\t\t${10|string,int,integer,number,bool,file|}\t${11|true,false|}\t"${12:Description}"',
    '//\t@Success\t\t${13|200,201,202,204|}\t\t${14|{object},{array},{string},{integer}|}\t${15:ResponseType}\t"${16:OK}"',
    '//\t@Header\t\t\t${17:200}\t\t{${18|string,integer|}}\t${19:X-Header-Name}\t"${20:Header description}"',
    '//\t@Failure\t\t${21|400,401,403,404,409,422,500|}\t\t{object}\t${22:response.ErrorResponse}\t"${23:Error}"',
    '//\t@Security\t\t${24:BearerAuth}',
    '//\t@Deprecated',
    '//\t@Router\t\t\t${25:/path} [${26|get,post,put,delete,patch,options,head|}]',
  ].join('\n');
}

/**
 * Generate a minimal Swaggo template with only the essential annotations.
 * Useful for quick documentation without all the optional fields.
 */
export function minimalTemplate(): string {
  return [
    '// ${1:handlerName} godoc',
    '//',
    '//\t@Summary\t\t${2:Summary}',
    '//\t@Description\t${3:Description}',
    '//\t@Tags\t\t\t${4:tag}',
    '//\t@Produce\t\tjson',
    '//\t@Success\t\t${5|200,201|}\t\t{object}\t${6:ResponseType}\t"${7:OK}"',
    '//\t@Failure\t\t${8|400,500|}\t\t{object}\t${9:response.ErrorResponse}\t"${10:Error}"',
    '//\t@Router\t\t\t${11:/path} [${12|get,post,put,delete|}]',
  ].join('\n');
}
