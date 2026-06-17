--[[
  Neovim LuaSnip Snippets for Swaggo Comment Generation.

  Installation:
    1. Copy this file to your LuaSnip snippets directory
       (e.g., ~/.config/nvim/luasnip/go.lua)
    2. Or require it from your LuaSnip config:
       require("luasnip.loaders.from_lua").load({ paths = "~/.config/nvim/luasnip/" })

  Usage:
    - Type the trigger (e.g., "sget") in a .go file and press <Tab> to expand
    - Use <Tab>/<S-Tab> to navigate between placeholders
]]

local ls = require("luasnip")
local s = ls.snippet
local t = ls.text_node
local i = ls.insert_node
local c = ls.choice_node

return {
  -- Full GET handler template
  s("sget", {
    t("// "), i(1, "handlerName"), t({" godoc", "//", ""}),
    t({"//\t@Summary\t\t"}), i(2, "Summary"), t({"", ""}),
    t({"//\t@Description\t"}), i(3, "Description"), t({"", ""}),
    t({"//\t@Tags\t\t\t"}), i(4, "tag"), t({"", ""}),
    t({"//\t@Produce\t\tjson", ""}),
    t({"//\t@Success\t\t200\t\t{object}\t"}), i(5, "dto.ResponseType"), t({'\t"'}), i(6, "OK"), t({'"', ""}),
    t({"//\t@Failure\t\t500\t\t{object}\tresponse.ErrorResponse\t\"Internal error\"", ""}),
    t({"//\t@Security\t\tBearerAuth", ""}),
    t({"//\t@Router\t\t\t"}), i(7, "/path"), t({" [get]"}),
  }),

  -- Full POST handler template with body param
  s("spost", {
    t("// "), i(1, "handlerName"), t({" godoc", "//", ""}),
    t({"//\t@Summary\t\t"}), i(2, "Summary"), t({"", ""}),
    t({"//\t@Description\t"}), i(3, "Description"), t({"", ""}),
    t({"//\t@Tags\t\t\t"}), i(4, "tag"), t({"", ""}),
    t({"//\t@Accept\t\t\tjson", ""}),
    t({"//\t@Produce\t\tjson", ""}),
    t({"//\t@Param\t\t\trequest\tbody\t\t"}), i(5, "dto.RequestType"), t({'\ttrue\t"Request payload"', ""}),
    t({"//\t@Success\t\t201\t\t{object}\t"}), i(6, "dto.ResponseType"), t({'\t"'}), i(7, "Created"), t({'"', ""}),
    t({"//\t@Failure\t\t400\t\t{object}\tresponse.ErrorResponse\t\"Bad request\"", ""}),
    t({"//\t@Failure\t\t500\t\t{object}\tresponse.ErrorResponse\t\"Internal error\"", ""}),
    t({"//\t@Security\t\tBearerAuth", ""}),
    t({"//\t@Router\t\t\t"}), i(8, "/path"), t({" [post]"}),
  }),

  -- GET with path param
  s("sgetid", {
    t("// "), i(1, "handlerName"), t({" godoc", "//", ""}),
    t({"//\t@Summary\t\t"}), i(2, "Summary"), t({"", ""}),
    t({"//\t@Description\t"}), i(3, "Description"), t({"", ""}),
    t({"//\t@Tags\t\t\t"}), i(4, "tag"), t({"", ""}),
    t({"//\t@Produce\t\tjson", ""}),
    t({"//\t@Param\t\t\t"}), i(5, "id"), t({"\tpath\t\t"}), i(6, "string"), t({'\ttrue\t"'}), i(7, "Resource ID"), t({'"', ""}),
    t({"//\t@Success\t\t200\t\t{object}\t"}), i(8, "dto.ResponseType"), t({'\t"'}), i(9, "OK"), t({'"', ""}),
    t({"//\t@Failure\t\t404\t\t{object}\tresponse.ErrorResponse\t\"Not found\"", ""}),
    t({"//\t@Security\t\tBearerAuth", ""}),
    t({"//\t@Router\t\t\t"}), i(10, "/resource/{id}"), t({" [get]"}),
  }),

  -- General/empty template with method choice
  s("swag", {
    t("// "), i(1, "handlerName"), t({" godoc", "//", ""}),
    t({"//\t@Summary\t\t"}), i(2, "Summary"), t({"", ""}),
    t({"//\t@Description\t"}), i(3, "Description"), t({"", ""}),
    t({"//\t@Tags\t\t\t"}), i(4, "tag"), t({"", ""}),
    t({"//\t@Accept\t\t\tjson", ""}),
    t({"//\t@Produce\t\tjson", ""}),
    t({"//\t@Success\t\t"}), c(5, { t("200"), t("201"), t("202") }), t({"\t\t{object}\t"}), i(6, "dto.ResponseType"), t({'\t"'}), i(7, "OK"), t({'"', ""}),
    t({"//\t@Failure\t\t500\t\t{object}\tresponse.ErrorResponse\t\"Internal error\"", ""}),
    t({"//\t@Router\t\t\t"}), i(8, "/path"), t({" ["}), c(9, { t("get"), t("post"), t("put"), t("delete") }), t({"]"}),
  }),

  -- DELETE handler template
  s("sdel", {
    t("// "), i(1, "handlerName"), t({" godoc", "//", ""}),
    t({"//\t@Summary\t\t"}), i(2, "Summary"), t({"", ""}),
    t({"//\t@Description\t"}), i(3, "Description"), t({"", ""}),
    t({"//\t@Tags\t\t\t"}), i(4, "tag"), t({"", ""}),
    t({"//\t@Produce\t\tjson", ""}),
    t({"//\t@Param\t\t\t"}), i(5, "id"), t({"\tpath\t\tstring\ttrue\t\"Resource ID\"", ""}),
    t({"//\t@Success\t\t200\t\t{object}\t"}), i(6, "dto.ResponseType"), t({'\t"Deleted"', ""}),
    t({"//\t@Failure\t\t404\t\t{object}\tresponse.ErrorResponse\t\"Not found\"", ""}),
    t({"//\t@Security\t\tBearerAuth", ""}),
    t({"//\t@Router\t\t\t"}), i(7, "/resource/{id}"), t({" [delete]"}),
  }),
}
