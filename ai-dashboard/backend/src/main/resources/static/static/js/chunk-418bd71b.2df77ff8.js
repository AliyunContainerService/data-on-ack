(window["webpackJsonp"]=window["webpackJsonp"]||[]).push([["chunk-418bd71b"],{1276:function(e,t,a){"use strict";var n=a("d784"),r=a("44e7"),o=a("825a"),i=a("1d80"),s=a("4840"),c=a("8aa5"),u=a("50c4"),l=a("14c3"),d=a("9263"),f=a("d039"),m=[].push,p=Math.min,h=4294967295,g=!f((function(){return!RegExp(h,"y")}));n("split",2,(function(e,t,a){var n;return n="c"=="abbc".split(/(b)*/)[1]||4!="test".split(/(?:)/,-1).length||2!="ab".split(/(?:ab)*/).length||4!=".".split(/(.?)(.?)/).length||".".split(/()()/).length>1||"".split(/.?/).length?function(e,a){var n=String(i(this)),o=void 0===a?h:a>>>0;if(0===o)return[];if(void 0===e)return[n];if(!r(e))return t.call(n,e,o);var s,c,u,l=[],f=(e.ignoreCase?"i":"")+(e.multiline?"m":"")+(e.unicode?"u":"")+(e.sticky?"y":""),p=0,g=new RegExp(e.source,f+"g");while(s=d.call(g,n)){if(c=g.lastIndex,c>p&&(l.push(n.slice(p,s.index)),s.length>1&&s.index<n.length&&m.apply(l,s.slice(1)),u=s[0].length,p=c,l.length>=o))break;g.lastIndex===s.index&&g.lastIndex++}return p===n.length?!u&&g.test("")||l.push(""):l.push(n.slice(p)),l.length>o?l.slice(0,o):l}:"0".split(void 0,0).length?function(e,a){return void 0===e&&0===a?[]:t.call(this,e,a)}:t,[function(t,a){var r=i(this),o=void 0==t?void 0:t[e];return void 0!==o?o.call(t,r,a):n.call(String(r),t,a)},function(e,r){var i=a(n,e,this,r,n!==t);if(i.done)return i.value;var d=o(e),f=String(this),m=s(d,RegExp),b=d.unicode,v=(d.ignoreCase?"i":"")+(d.multiline?"m":"")+(d.unicode?"u":"")+(g?"y":"g"),y=new m(g?d:"^(?:"+d.source+")",v),x=void 0===r?h:r>>>0;if(0===x)return[];if(0===f.length)return null===l(y,f)?[f]:[];var O=0,D=0,T=[];while(D<f.length){y.lastIndex=g?D:0;var j,C=l(y,g?f:f.slice(D));if(null===C||(j=p(u(y.lastIndex+(g?0:D)),f.length))===O)D=c(f,D,b);else{if(T.push(f.slice(O,D)),T.length===x)return T;for(var w=1;w<=C.length-1;w++)if(T.push(C[w]),T.length===x)return T;D=O=j}}return T.push(f.slice(O)),T}]}),!g)},"13d5":function(e,t,a){"use strict";var n=a("23e7"),r=a("d58f").left,o=a("a640"),i=a("ae40"),s=o("reduce"),c=i("reduce",{1:0});n({target:"Array",proto:!0,forced:!s||!c},{reduce:function(e){return r(this,e,arguments.length,arguments.length>1?arguments[1]:void 0)}})},"23ba":function(e,t,a){"use strict";a.d(t,"h",(function(){return l})),a.d(t,"g",(function(){return m})),a.d(t,"a",(function(){return p})),a.d(t,"e",(function(){return h})),a.d(t,"b",(function(){return g})),a.d(t,"f",(function(){return b})),a.d(t,"c",(function(){return y})),a.d(t,"d",(function(){return x})),a.d(t,"i",(function(){return O}));var n=a("b85c"),r=a("2909"),o=a("3835"),i=(a("4fad"),a("ac1f"),a("466d"),a("1276"),a("a15b"),a("fb6a"),a("b0c0"),a("e9c4"),a("99af"),a("a434"),a("d3b7"),a("159b"),a("c740"),a("caad"),a("2532"),a("ed08")),s=a("61f7"),c=a("b775");function u(e,t){if(void 0!==e)for(var a=0,n=Object.entries(e);a<n.length;a++){var r=n[a],o=r[0];t.root.indexOf(o)<0&&t.root.push(o)}}function l(e){if(Object(i["b"])(e)||Object(i["b"])(e.memory))return e;var t=e.memory,a=/(^(\d{1,})\.?(\d{0,})$)|(^N\/A$)/g,n=t.match(a);return null!==n&&(e.memory=t+"M"),e}function d(e){var t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:".",a=e.split(t),n="";return a.length>1&&(n=a.slice(0,a.length-1).join(t)),[n,a[a.length-1]]}function f(e,t){return Object(i["b"])(e)?t:e+"."+t}function m(e,t){if(Object(i["b"])(e)||Object(i["b"])(e.name))return t;var a=JSON.parse(JSON.stringify(e));if(a.name=f(e.prefix,e.name),a.min=l(a.min),a.max=l(a.max),t=a,Object(i["b"])(e.children))return t;t.children=[];for(var n=0;n<e.children.length;n++){var r=e.children[n];t.children.push(m(r,null))}return t}function p(e){var t=e.k8sVersion;return Object(s["b"])(t,"1.20.0")>=0}function h(e,t,a){void 0===t&&(t={root:[]}),void 0===a&&(a={root:[]});var n={};if(void 0===e.name)return[n,t,a];var s=d(e.name),c=Object(o["a"])(s,2),l=c[0],f=c[1];if(n.name=f,n.prefix=l,n.min=e.min,u(n.min,t),n.max=e.max,u(n.max,t),void 0!==e.namespaces&&(n.namespaces=e.namespaces,a.root=[].concat(Object(r["a"])(a.root),Object(r["a"])(n.namespaces))),!Object(i["b"])(e.children)){void 0===n.children&&(n.children=[]);for(var m=0;m<e.children.length;m++){var p,g=e.children[m],b=h(g,t,a),v=Object(o["a"])(b,3);p=v[0],t=v[1],a=v[2],n.children.push(p)}}return[n,t,a]}function g(e,t){var a=arguments.length>2&&void 0!==arguments[2]?arguments[2]:"",n=e.children||[],r=a.split(".");r.splice(0,1);var o=0;if(r.length>0)while(o<r.length)n.forEach((function(e){e.name===r[o]&&(n=e.children||[],o++)}));var i=n.findIndex((function(e){return e.name===t}));return i<0?(console.log("not found node name:",t,a),e):(n.splice(i,1),e)}function b(){var e,t=arguments.length>0&&void 0!==arguments[0]?arguments[0]:"prefix",a=arguments.length>1?arguments[1]:void 0,n=arguments.length>2?arguments[2]:void 0,r=function a(r){r.name===n||(r.name||"").includes(n)?e="prefix"===t?r.prefix:r:r.children&&r.children.forEach((function(e){return a(e)}))};return r(a),e}var v=function e(t){var a=arguments.length>1&&void 0!==arguments[1]?arguments[1]:[];if((!t.children||t.children&&0===t.children.length)&&a.push(f(t.prefix,t.name)),t.children){var r,o=Object(n["a"])(t.children);try{for(o.s();!(r=o.n()).done;){var i=r.value;e(i,a)}}catch(s){o.e(s)}finally{o.f()}}};function y(){var e=arguments.length>0&&void 0!==arguments[0]?arguments[0]:{},t=[];return e.spec&&e.spec.root&&v(e.spec.root,t),t}function x(e){return Object(c["a"])({url:"/group/list",method:"get",params:e})}function O(e,t,a,n,r){return Object(i["b"])(n)&&(n=a),Object(c["a"])({url:"/group/update",method:"put",params:{oldNodeName:a,newNodeName:n,prefix:r,action:t},data:e})}},2909:function(e,t,a){"use strict";a.d(t,"a",(function(){return c}));var n=a("6b75");function r(e){if(Array.isArray(e))return Object(n["a"])(e)}a("a4d3"),a("e01a"),a("d3b7"),a("d28b"),a("3ca3"),a("ddb0"),a("a630");function o(e){if("undefined"!==typeof Symbol&&null!=e[Symbol.iterator]||null!=e["@@iterator"])return Array.from(e)}var i=a("06c5");function s(){throw new TypeError("Invalid attempt to spread non-iterable instance.\nIn order to be iterable, non-array objects must have a [Symbol.iterator]() method.")}function c(e){return r(e)||o(e)||Object(i["a"])(e)||s()}},"466d":function(e,t,a){"use strict";var n=a("d784"),r=a("825a"),o=a("50c4"),i=a("1d80"),s=a("8aa5"),c=a("14c3");n("match",1,(function(e,t,a){return[function(t){var a=i(this),n=void 0==t?void 0:t[e];return void 0!==n?n.call(t,a):new RegExp(t)[e](String(a))},function(e){var n=a(t,e,this);if(n.done)return n.value;var i=r(e),u=String(this);if(!i.global)return c(i,u);var l=i.unicode;i.lastIndex=0;var d,f=[],m=0;while(null!==(d=c(i,u))){var p=String(d[0]);f[m]=p,""===p&&(i.lastIndex=s(u,o(i.lastIndex),l)),m++}return 0===m?null:f}]}))},"4ec9":function(e,t,a){"use strict";var n=a("6d61"),r=a("6566");e.exports=n("Map",(function(e){return function(){return e(this,arguments.length?arguments[0]:void 0)}}),r)},"4fad":function(e,t,a){var n=a("23e7"),r=a("6f53").entries;n({target:"Object",stat:!0},{entries:function(e){return r(e)}})},"6f53":function(e,t,a){var n=a("83ab"),r=a("df75"),o=a("fc6a"),i=a("d1e7").f,s=function(e){return function(t){var a,s=o(t),c=r(s),u=c.length,l=0,d=[];while(u>l)a=c[l++],n&&!i.call(s,a)||d.push(e?[a,s[a]]:s[a]);return d}};e.exports={entries:s(!0),values:s(!1)}},"7daf":function(e,t,a){"use strict";a.r(t);var n=function(){var e=this,t=e.$createElement,a=e._self._c||t;return a("div",{staticClass:"app-container"},[a("div",{staticClass:"filter-container"},[a("el-input",{staticClass:"filter-item",staticStyle:{width:"200px"},attrs:{placeholder:e.$t("quota.name")},model:{value:e.listQuery.name,callback:function(t){e.$set(e.listQuery,"name",t)},expression:"listQuery.name"}}),a("el-button",{staticClass:"filter-item",staticStyle:{"margin-left":"10px"},attrs:{disabled:void 0===e.tableData,type:"primary",icon:"el-icon-search"},on:{click:e.handleFilter}},[e._v(" "+e._s(e.$t("quota.search"))+" ")]),a("el-button",{staticClass:"filter-item",staticStyle:{"margin-left":"10px"},attrs:{disabled:e.isOnlyRootNode(),type:"primary",icon:"el-icon-edit"},on:{click:e.appendResourceType}},[e._v(" "+e._s(e.$t("quota.changeResourceType"))+" ")]),a("el-button",{staticClass:"fr",staticStyle:{"margin-left":"10px"},attrs:{type:"default",icon:"el-icon-refresh-left"},on:{click:e.getList}},[e._v(" "+e._s(e.$t("job.refresh"))+" ")])],1),a("div",{staticClass:"custom-table-container"},[a("div",{staticClass:"block"},[a("el-table",{ref:"table",attrs:{data:e.tableData,"row-key":e.getTableRowKey,border:"","cell-class-name":"quota-group-table",indent:36,"tree-props":{children:"children",hasChildren:"hasChildren"},"highlight-current-row":"","expand-row-keys":e.expandRows,"default-expand-all":""}},[a("el-table-column",{attrs:{label:e.$t("quota.name"),prop:"name",width:"300",fixed:""}}),a("el-table-column",{attrs:{label:e.$t("quota.namespace"),width:"150"},scopedSlots:e._u([{key:"default",fn:function(t){return e._l(t.row.namespaces||[],(function(t){return a("span",{key:t},[e._v(e._s(t)),a("br")])}))}}])}),e._l(e.minColumns,(function(t,n){return a("el-table-column",{key:n,attrs:{label:t,"min-width":"100","show-overflow-tooltip":""},scopedSlots:e._u([{key:"default",fn:function(n){return[a("span",{staticStyle:{color:"#ccc"}},[e._v("[ ")]),a("span",[e._v(e._s(n.row.min[t]))]),a("span",[e._v(" , ")]),a("span",[e._v(e._s(n.row.max[t]))]),a("span",{staticStyle:{color:"#ccc"}},[e._v(" ]")])]}}],null,!0)})})),a("el-table-column",{attrs:{align:"right",fixed:"right","min-width":"150",label:e.$t("quota.operator")},scopedSlots:e._u([{key:"default",fn:function(t){return[e.canAddNode(t.row)?a("el-button",{attrs:{type:"text"},on:{click:function(){return e.append(t.row)}}},[e._v(" "+e._s(e.$t("quota.add"))+" ")]):e._e(),e.canUpdate(t.row)?a("el-button",{attrs:{type:"text"},on:{click:function(){return e.update(t.row)}}},[e._v(" "+e._s(e.$t("quota.edit"))+" ")]):e._e(),e.canDeleteNode(t.row)?a("el-button",{attrs:{type:"text"},on:{click:function(){return e.remove(t.row)}}},[e._v(" "+e._s(e.$t("quota.delete"))+" ")]):e._e()]}}])})],2)],1),a("el-dialog",{attrs:{title:e.getDialogueTitle(e.dialogStatus),visible:e.dialogFormVisible},on:{"update:visible":function(t){e.dialogFormVisible=t}}},[a("el-form",{ref:"dataForm",staticStyle:{"margin-left":"20px"},attrs:{model:e.curData,"label-position":"left","label-width":"0px"}},[a("el-form-item",{attrs:{prop:"name","label-width":"120px",label:e.$t("quota.name"),rules:{required:!0,validator:e.quotaNameValidate,trigger:"blur"}}},[a("el-input",{attrs:{placeholder:e.getFormatExample("name"),disabled:e.isDisableEditName(e.dialogStatus)},model:{value:e.curData.name,callback:function(t){e.$set(e.curData,"name",t)},expression:"curData.name"}})],1),e.canChangeNamespace(e.curData,e.dialogStatus)?a("el-form-item",{attrs:{"label-width":"120px",label:e.$t("quota.namespace"),prop:"namespaces"}},[a("el-select",{attrs:{placeholder:e.$t("quota.placeholder.namespace"),multiple:e.isMultiple(),filterable:""},model:{value:e.curData.namespaces,callback:function(t){e.$set(e.curData,"namespaces",t)},expression:"curData.namespaces"}},e._l(e.namespaceList,(function(e,t){return a("el-option",{key:"namespaceitem"+t,attrs:{label:e,value:e}})})),1)],1):e._e(),void 0!==e.curData.quotaConfigs?a("el-card",[a("div",{attrs:{slot:"header"},slot:"header"},[a("el-row",[a("el-col",{attrs:{span:10}},[e._v(e._s(e.$t("quota.resourceType")))]),a("el-col",{attrs:{span:5}},[e._v(e._s(e.$t("quota.min")))]),a("el-col",{attrs:{span:5}},[e._v(e._s(e.$t("quota.max")))]),a("el-col",{attrs:{span:2}},[e._v(e._s(e.$t("quota.comment")))]),"changeResourceType"===e.dialogStatus?a("el-col",{attrs:{span:1}},[e._v(e._s(e.$t("quota.add")))]):e._e(),"changeResourceType"===e.dialogStatus?a("el-col",{attrs:{span:1}},[a("i",{staticClass:"el-icon-circle-plus-outline",staticStyle:{color:"green"},on:{click:e.appendOtherResource}})]):e._e(),"changeResourceType"!==e.dialogStatus?a("el-col",{attrs:{span:2}}):e._e()],1)],1),e._l(e.curData.quotaConfigs,(function(t,n){return a("div",{key:"quotaConfig"+n},[a("el-row",{staticClass:"row-bg",staticStyle:{"margin-bottom":"7px"},attrs:{type:"flex",justify:"space-between"}},[a("el-col",{attrs:{span:10}},[a("el-form-item",{attrs:{prop:"quotaConfigs."+n+".type",rules:{required:!0,validator:e.resourceTypeValidator,trigger:"blur"}}},[e.isDefaultResourceType(t.type,n)?a("el-input",{attrs:{placeholder:t.type,disabled:!e.isResourceTypeEditable(e.dialogStatus,t.type,n)}}):a("el-input",{attrs:{placeholder:t.type,disabled:"changeResourceType"!==e.dialogStatus},model:{value:t.type,callback:function(a){e.$set(t,"type",a)},expression:"quotaConfig.type"}})],1)],1),a("el-col",{staticStyle:{"margin-left":"10px"},attrs:{span:5}},[a("el-form-item",{attrs:{prop:"quotaConfigs."+n+".min",rules:{required:!1,validator:e.quotaFormatValidator,trigger:"blur"}}},[a("el-input",{attrs:{placeholder:e.getFormatExample(t.type)},model:{value:t.min,callback:function(a){e.$set(t,"min",a)},expression:"quotaConfig.min"}})],1)],1),a("el-col",{staticStyle:{"margin-left":"10px"},attrs:{span:5}},[a("el-form-item",{attrs:{prop:"quotaConfigs."+n+".max",rules:{required:!1,validator:e.quotaFormatValidator,trigger:"blur"}}},[a("el-input",{attrs:{placeholder:e.getFormatExample(t.type)},model:{value:t.max,callback:function(a){e.$set(t,"max",a)},expression:"quotaConfig.max"}})],1)],1),e.isEmptyWrapper(e.getDefaultQuotaComments()[t.type])?a("el-col",{staticStyle:{"margin-left":"10px","margin-top":"14px"},attrs:{span:2}}):a("el-col",{staticStyle:{"margin-left":"10px","margin-top":"14px"},attrs:{span:2}},[e._v(" "+e._s(e.isEmptyWrapper(e.getDefaultQuotaComments()[t.type]["shortComments"])?"":e.getDefaultQuotaComments()[t.type]["shortComments"])+" "),e.isEmptyWrapper(e.getDefaultQuotaComments()[t.type]["helpUrl"])?e._e():a("i",{staticClass:"el-icon-info",attrs:{title:e.getDefaultQuotaComments()[t.type]["longComments"]},on:{click:function(a){return e.clickComments(t.type)}}})]),"changeResourceType"===e.dialogStatus?a("el-col",{staticStyle:{"margin-left":"10px"},attrs:{span:1}}):e._e(),e.isResourceTypeEditable(e.dialogStatus,t.type,n)?a("el-col",{attrs:{span:1}},[a("i",{staticClass:"el-icon-delete",staticStyle:{color:"red","margin-top":"14px"},on:{click:function(t){return e.removeOtherResource(n)}}})]):a("el-col",{attrs:{span:1}}),"changeResourceType"!==e.dialogStatus?a("el-col",{attrs:{span:2}}):e._e()],1)],1)}))],2):e._e()],1),a("div",{staticClass:"dialog-footer",attrs:{slot:"footer"},slot:"footer"},[a("el-button",{on:{click:function(t){e.dialogFormVisible=!1}}},[e._v(" "+e._s(e.$t("quota.cancel"))+" ")]),a("el-button",{attrs:{type:"primary"},on:{click:function(t){return e.saveClickRouter(e.dialogStatus)}}},[e._v(" "+e._s(e.$t("quota.save"))+" ")])],1)],1)],1)])},r=[],o=a("b85c"),i=a("3835"),s=a("2909"),c=(a("a9e3"),a("99af"),a("ac1f"),a("466d"),a("13d5"),a("d3b7"),a("7db0"),a("b0c0"),a("4de4"),a("e9c4"),a("d81d"),a("b64b"),a("4fad"),a("a15b"),a("a434"),a("159b"),a("c1f9"),a("4ec9"),a("3ca3"),a("ddb0"),a("23ba")),u=a("b775");function l(e){return Object(u["a"])({url:"/k8s/namespace/list",method:"get",params:e})}var d=a("ed08"),f=a("5f87"),m={data:function(){var e=this,t={name:"",prefix:"",namespaces:[],min:{},max:{},children:[]};return{validateQuotaLogic:function(e){var t=e.min,a=e.max;if(!Object(d["b"])(t)&&!Object(d["b"])(a))try{var n=!1;if("N/A"===t&&"N/A"!==a&&(n=!0),Number(e.min)>Number(e.max)&&(n=!0),n)return this.$notify({title:this.$t("quota.validator.minMaxRelationError"),dangerouslyUseHTMLString:!0,message:"".concat(e.type," min>max min:").concat(t," max:").concat(a),type:"error",duration:2e3}),!1}catch(r){return!0}return!0},validateQuotaFormat:function(e,t){var a=/(^\d+$)|(^N\/A$)/g,n=this.getDefaultQuotaComments();if(void 0!==n[e]&&void 0!==n[e].format&&(a=n[e].format),Object(d["b"])(t))return this.$notify({title:this.$t("quota.validator.formatError"),message:"Name:"+e+" empty",dangerouslyUseHTMLString:!0,type:"error",duration:2e3}),!1;var r=t.match(a);return null!==r||(this.$notify({title:this.$t("quota.validator.formatError"),dangerouslyUseHTMLString:!0,message:"Name:"+e+" quota:"+t+", should be:"+a,type:"error",duration:2e3}),!1)},quotaNameValidate:function(t,a,n){if(e.isDisableEditName(e.dialogStatus))n();else{var r=t.field.split(".")[0];if(!e.validateQuotaFormat(r,a))return Object(d["b"])(e.getDefaultQuotaComments()[r])||Object(d["b"])(e.getDefaultQuotaComments()[r].formatExample)?void n(new Error(e.$t("quota.validator.formatError"))):void n(new Error(e.getDefaultQuotaComments()[r].formatExample));var o=e.editRow.prefix||"",i=e.$refs.table.tableData,s=o.split("."),c=s.reduce((function(e,t){var a=e.find((function(e){return e.name===t}));return a?a.children:[]}),i);""===o&&(c=i);var u=(c||[]).filter((function(e){return e.name===a}));if("addQuota"===e.dialogStatus){var l=(c||[]).find((function(t){return t.name===e.editRowName}));u=l?(l.children||[]).filter((function(e){return e.name===a})):[]}"editQuota"===e.dialogStatus&&e.editRowName&&(u=(c||[]).filter((function(t){return t.name===a&&t.name!==e.editRowName}))),u.length>0?n(new Error(e.$t("quota.validator.nameRepeat"))):n()}},resourceTypeValidator:function(t,a,n){var r=Number(t.field.split(".")[1]);e.validateQuotaFormat("resourceType",a)?e.resourceTypes.indexOf(a)>=0&&e.resourceTypes.indexOf(a)!==r?n(new Error(e.$t("quota.validator.resourceTypeNotUnique"))):n():n(new Error(e.$t("quota.validator.formatError")+":"+e.getDefaultQuotaComments()["resourceType"].formatExample))},quotaFormatValidator:function(t,a,n){var r=t.field.split(".")[0],o=Number(t.field.split(".")[1]),i="",s=e.curData.quotaConfigs;if("quotaConfigs"===r&&(i=s[o].type),!e.validateQuotaFormat(i,a))return Object(d["b"])(e.getDefaultQuotaComments()[i])||Object(d["b"])(e.getDefaultQuotaComments()[i].formatExample)?void n(new Error(e.$t("quota.validator.mustBeNumber"))):void n(new Error(e.getDefaultQuotaComments()[i].formatExample));e.validateQuotaLogic(s[o])?n():n(new Error(e.$t("quota.validator.minMaxRelationError")))},k8sElasticQuotaTree:void 0,treeData:[],tableData:[],listQuery:{name:""},namespaceList:[],resourceTypes:["cpu","memory","nvidia.com/gpu","aliyun.com/gpu","aliyun.com/gpu-mem"],defaultResourceTypes:["cpu","memory","nvidia.com/gpu","aliyun.com/gpu","aliyun.com/gpu-mem"],allNamespaceList:[],dialogFormVisible:!1,dialogStatus:"",curNode:void 0,curData:JSON.parse(JSON.stringify(t)),TreeAction:{CreateTree:"createTree",AddNode:"addNode",DeleteNode:"deleteNode",UpdateNode:"updateNode",UpdateResourceType:"updateResourceType"},defaultCurData:t,defaultProps:{children:"children",label:"name",id:"name"},expandRows:[],minColumns:[]}},created:function(){this.getList(),Array.prototype.equals||(console.warn("Overriding existing Array.prototype.equals. Possible causes: New API defines the method, there's a framework conflict or you've got double inclusions in your code."),Array.prototype.equals=d["c"],Object.defineProperty(Array.prototype,"equals",{enumerable:!1}))},methods:{getDefaultQuotaComments:function(){return{name:{longComments:"",formatExample:"请输入名称（可包含数字、字母、（_），不能以（_）开头）",format:/^([a-zA-Z]{1})([_a-zA-Z0-9]{0,64})$/g,shortComments:"name"},resourceType:{longComments:"",formatExample:"资源类型只能是以字母开头，包含字母、数字、（-）、（_）、（.）、（/）的字符串",format:/^([a-zA-Z]{1})([\.\/\-_.a-zA-Z0-9]{0,64})$/g,shortComments:"resourceType"},cpu:{longComments:"",formatExample:"1,0.1,100m",format:/^((\d+)\.?(\d{0,})|N\/A)(m{0,1})$/g,shortComments:"core",helpUrl:"https://kubernetes.io/zh/docs/concepts/configuration/manage-resources-containers/#meaning-of-cpu"},memory:{longComments:"",format:/^((\d+)\.?(\d{0,})|N\/A)([EPTGMK]{0,1}[i]{0,1})$/g,formatExample:"128,129M,123Mi",shortComments:"M",helpUrl:"https://kubernetes.io/zh/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory"},"aliyun.com/gpu-mem":{longComments:"gpu memory",format:/^(\d+|N\/A)([G]{0,1}[i]{0,1})$/g,shortComments:"G",helpUrl:"https://help.aliyun.com/document_detail/191152.html?spm=a2c4g.11186623.6.686.54691effXkB41b"},"nvidia.com/gpu":{longComments:"gpu",formatExp:/(^\d+$)|(^N\/A$)/g,shortComments:this.$t("quota.gpuCard"),helpUrl:void 0},"aliyun.com/gpu":{longComments:"gpu topology",formatExp:/(^\d+$)|(^N\/A$)/g,shortComments:this.$t("quota.gpuCard"),helpUrl:"https://help.aliyun.com/document_detail/190482.html"},"aliyun.com/gpu-core.percentage":{longComments:"gpu core percentage",formatExp:/(^\d+$)|(^N\/A$)/g,shortComments:this.$t("quota.gpuCard"),helpUrl:"https://help.aliyun.com/document_detail/424668.html"}}},getDialogueTitle:function(e){return this.$t("quota."+e)},filterNamespace:function(e){if(void 0===this.k8sElasticQuotaTree||void 0===this.k8sElasticQuotaTree.spec)return e.map((function(e){return e.name}));var t=Object(c["e"])(this.k8sElasticQuotaTree.spec.root)[2].root;return e.filter((function(e){return t.indexOf(e.name)<0})).map((function(e){return e.name}))},getFormatExample:function(e){var t=this.$t("quota.validator.mustBeNumber"),a=this.getDefaultQuotaComments();return void 0===a[e]||Object(d["b"])(a[e].formatExample)?t:a[e].formatExample},isMultiple:function(){var e=Object(f["a"])();return Object(c["a"])(e)},getNamespace:function(){var e=this;l().then((function(t){e.allNamespaceList=Object(s["a"])(t.data),e.namespaceList=e.filterNamespace(e.allNamespaceList)})).catch((function(t){e.$notify({title:e.$t("quota.retry"),dangerouslyUseHTMLString:!0,message:e.$t("quota.exception.getNamespace")+"Error:"+t,type:"error",duration:2e3})}))},getList:function(){var e=this;this.listLoading=!0,Object(c["d"])().then((function(t){var a,n;e.listQuery.name="",e.treeData=[];var r=JSON.parse(JSON.stringify(e.defaultCurData));if(r.name="root",e.tableData=Object(d["b"])(t.data)?[r]:[null===(a=t.data)||void 0===a||null===(n=a.spec)||void 0===n?void 0:n.root],e.tableData[0]&&e.tableData[0].min&&(e.minColumns=Object.keys(e.tableData[0].min||{})),e.k8sElasticQuotaTree=t.data,void 0===e.k8sElasticQuotaTree&&(e.k8sElasticQuotaTree={spec:{root:{}}}),Object(d["b"])(t.data))e.treeData.push(r);else{var o=Object(c["e"])(e.k8sElasticQuotaTree.spec.root),u=Object(i["a"])(o,3),l=u[0],f=u[1];u[2];Object(d["b"])(l)?e.treeData.push(r):(e.treeData.push(l),e.resourceTypes=[].concat(Object(s["a"])(e.defaultResourceTypes),Object(s["a"])(f.root)).filter((function(e,t,a){return a.indexOf(e)===t})))}})).catch((function(t){e.$notify({title:e.$t("quota.retry"),dangerouslyUseHTMLString:!0,message:e.$t("quota.exception.getElasticQuotaTree")+":"+t,type:"error",duration:2e3})})).finally((function(){e.listLoading=!1}))},packKVObjectToString:function(e){for(var t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:",",a=arguments.length>2&&void 0!==arguments[2]?arguments[2]:":",n=[],r=0,o=Object.entries(e);r<o.length;r++){var s=Object(i["a"])(o[r],2),c=s[0],u=s[1];n.push([c,u].join(a))}return n.join(t)},handleFilter:function(){var e=this.$refs.table.tableData,t=Object(c["f"])("prefix",e[0],this.listQuery.name);this.expandRows=(t||"").split(".");var a=Object(c["f"])("currentRow",e[0],this.listQuery.name);this.$refs.table.setCurrentRow(a),this.$nextTick((function(){var e=document.querySelector(".current-row");a&&e&&e.scrollIntoView({block:"start",behavior:"smooth"})}))},treeFilter:function(e,t){return!(!t||Object(d["b"])(t.name))&&(!e||-1!==t.name.indexOf(e))},appendOtherResource:function(){this.curData.quotaConfigs.push({type:"",min:"0",max:"N/A"})},removeOtherResource:function(e){this.curData.quotaConfigs.splice(e,1)},updateTreeResourceType:function(e,t,a){function n(e,t,a,n){var r={};return t.forEach((function(t){var o=n[t],i=e[t];!Object(d["b"])(o)&&Object(d["b"])(i)&&(i=e[o]),r[t]=i||a})),r}if(!Object(d["b"])(e)&&(Object(d["b"])(e.min)||(e.min=n(e.min,t,"0",a)),Object(d["b"])(e.max)||(e.max=n(e.max,t,"N/A",a)),!Object(d["b"])(e.children)))for(var r=0;r<e.children.length;r++)this.updateTreeResourceType(e.children[r],t,a)},notifyDependResponse:function(e,t,a){1e4===e.code?this.$notify({title:this.$t("quota.success"),message:this.$t(t),type:"success",duration:2e3}):this.$notify({title:this.$t("quota.error"),dangerouslyUseHTMLString:!0,message:this.$t(a)+":"+e.data,type:"error",duration:2e3})},doAppendResourceType:function(){var e=this;this.$refs["dataForm"].validate((function(t){if(t){var a=Object(s["a"])(e.resourceTypes),n=e.curData.quotaConfigs.map((function(e){return e.type})),r={};n.forEach((function(e,t){r[e]=t<a.length?a[t]:void 0})),a.equals(n)||(e.resourceTypes=Object(s["a"])(n),e.updateTreeResourceType(e.tableData[0],n,r)),e.genK8sQuotaConfig(e.curData),Object.assign(e.editRow,{min:e.curData.min,max:e.curData.max,namespaces:Object(d["a"])(e.curData.namespaces)?e.curData.namespaces:[e.curData.namespaces]});var o=e.TreeAction.UpdateResourceType,i=e.curData.name,u=Object.assign({},e.k8sElasticQuotaTree,{spec:{root:e.tableData[0]}});Object(c["i"])(u,o,i,i,"").then((function(t){e.getList(),e.notifyDependResponse(t,"quota.ok.changeResourceType","quota.exception.changeResourceType")})).catch((function(t){e.getList(),e.$notify({title:e.$t("quota.retry"),dangerouslyUseHTMLString:!0,message:e.$t("quota.exception.changeResourceType")+":"+t,type:"error",duration:2e3})})).finally((function(){e.dialogFormVisible=!1}))}}))},appendResourceType:function(){var e=this;this.dialogStatus="changeResourceType";var t=this.tableData[0];this.editRow=this.tableData[0],this.curData=this.resetCurDataTo(t),this.dialogFormVisible=!0,this.$nextTick((function(){e.$refs["dataForm"].clearValidate(),e.listLoading=!1}))},udpateElasticeQuotaTree:function(e,t){var a=Object(c["g"])(e[0],a);return t.spec.root=a,t},getTableRowKey:function(e){return(e.prefix||"")+e.name},update:function(e){var t=this;this.editRow=e,this.editRowName=e.name,this.dialogStatus="editQuota",this.curData=this.resetCurDataTo(e),(void 0===this.allNamespaceList||this.allNamespaceList.length<1)&&this.getNamespace(),this.dialogFormVisible=!0,this.$nextTick((function(){t.$refs["dataForm"].clearValidate(),t.listLoading=!1}))},doUpdate:function(){var e=this;this.$refs["dataForm"].validate((function(t){if(t){e.genK8sQuotaConfig(e.curData);var a=e.editRow.name,n=e.curData.name,r=e.editRow.prefix||"",o=1===e.tableData.length&&Object(d["b"])(e.tableData[0].children)?"createTree":e.TreeAction.UpdateNode;Object.assign(e.editRow,{name:e.curData.name,namespaces:Object(d["a"])(e.curData.namespaces)?e.curData.namespaces:[e.curData.namespaces],min:e.curData.min,max:e.curData.max});var i=Object.assign({},e.k8sElasticQuotaTree,{spec:{root:e.tableData[0]}});Object(c["i"])(i,o,a,n,r).then((function(t){e.getList(),e.notifyDependResponse(t,"quota.ok.updateSuccess","quota.exception.updateQuota")})).catch((function(t){e.$notify({title:e.$t("quota.retry"),dangerouslyUseHTMLString:!0,message:t,type:"error",duration:2e3}),e.getList()})).finally((function(){e.dialogFormVisible=!1}))}}))},isOnlyRootNode:function(){return 1===this.treeData.length&&Object(d["b"])(this.treeData[0].children)},doAppend:function(){var e=this;this.$refs["dataForm"].validate((function(t){if(t){var a=1===e.tableData.length&&Object(d["b"])(e.tableData[0].children)?"createTree":"addNode",n=e.curData.name,r=e.editRow.prefix?[e.editRow.prefix,e.editRow.name].join("."):e.editRow.name;e.genK8sQuotaConfig(e.curData);var o={name:n,prefix:r,min:e.curData.min,max:e.curData.max,namespaces:Object(d["a"])(e.curData.namespaces)?e.curData.namespaces:[e.curData.namespaces]};e.editRow.children=(e.editRow.children||[]).concat(o);var i=Object.assign({},e.k8sElasticQuotaTree,{spec:{root:e.tableData[0]}});Object(c["i"])(i,a,n,n,r).then((function(t){e.dialogFormVisible=!1,e.getList(),e.notifyDependResponse(t,"quota.ok.addSuccess","quota.exception.addQuota")})).catch((function(t){e.getList(),e.$notify({title:e.$t("quota.retry"),dangerouslyUseHTMLString:!0,message:t,type:"error",duration:2e3})})).finally((function(){e.dialogFormVisible=!1}))}}))},append:function(e){var t=this;this.editRow=e,this.editRowName=e.name,this.dialogStatus="addQuota",this.curData=this.resetCurDataTo(this.defaultCurData,e),void 0===this.allNamespaceList||this.allNamespaceList.length<1?this.getNamespace():this.namespaceList=this.filterNamespace(this.allNamespaceList),this.dialogFormVisible=!0,this.$nextTick((function(){t.$refs["dataForm"].clearValidate(),t.listLoading=!1}))},remove:function(e){var t=this,a=e.name,n=e.prefix,r=Object(c["b"])(this.tableData[0],a,n),o=Object.assign({},this.k8sElasticQuotaTree,{spec:{root:r}});Object(c["i"])(o,this.TreeAction.DeleteNode,a,null,n).then((function(e){t.getList(),t.notifyDependResponse(e,"quota.ok.deleteSuccess","quota.exception.deleteQuota")})).catch((function(e){t.getList(),t.$notify({title:t.$t("quota.retry"),dangerouslyUseHTMLString:!0,message:t.$t("quota.exception.deleteQuota")+":"+e,type:"error",duration:2e3})}))},clickComments:function(e){var t=this.getDefaultQuotaComments()[e];if(!Object(d["b"])(t)){var a=t.helpUrl;Object(d["b"])(a)||window.open(a,"_blank")}},isLeaf:function(e){return Object(d["b"])(e.children)||e.children.length<1},isDisableEditName:function(e){return"changeResourceType"===e},canUpdate:function(e){return void 0!==e.name},canDeleteNode:function(e){if(void 0===e.name||this.isOnlyRootNode())return!1;if(this.isLeaf(e)){if(this.isLeaf(this.tableData[0]))return!1;if(void 0===e.namespaces||e.namespaces.length<1)return!0}return!1},canAddNode:function(e){return!this.isLeaf(e)||(void 0===e.namespaces||e.namespaces.length<1)},canChangeNamespace:function(e,t){return(!this.isOnlyRootNode()||"addQuota"===t)&&this.isLeaf(e)},isEmptyWrapper:function(e){return Object(d["b"])(e)},genK8sQuotaConfig:function(e){return e.min=Object.fromEntries(new Map(e.quotaConfigs.map((function(e){return[e.type,e.min]})))),e.max=Object.fromEntries(new Map(e.quotaConfigs.map((function(e){return[e.type,e.max]})))),Object(d["b"])(e.min.memory)||(Object(c["h"])(e.min),Object(c["h"])(e.max)),e},resetCurDataTo:function(e,t){var a=JSON.parse(JSON.stringify(e));if(void 0===a.quotaConfigs&&(a.quotaConfigs=[]),void 0===a.otherResources&&(a.otherResources=[]),a.quotaConfigs.length<1){var n,r=Object(o["a"])(this.resourceTypes.entries());try{for(r.s();!(n=r.n()).done;){var i=n.value,s=i[1],c=!Object(d["b"])(a.min)&&a.min[s]?a.min[s]:"0",u=!Object(d["b"])(a.max)&&a.max[s]?a.max[s]:t&&t.max&&t.max[s]||"N/A";a.quotaConfigs.push({type:s,min:c,max:u})}}catch(l){r.e(l)}finally{r.f()}}return a},saveClickRouter:function(e){var t=this;"addQuota"===e?this.doAppend():"changeResourceType"===e?this.doAppendResourceType():this.doUpdate(),this.$nextTick((function(){t.$refs["dataForm"].clearValidate()}))},isDefaultResourceType:function(e,t){return this.defaultResourceTypes.indexOf(e)>=0&&t<this.defaultResourceTypes.length},isResourceTypeEditable:function(e,t,a){var n=this.isDefaultResourceType(t,a);return"changeResourceType"===e&&!n},isNewResourceType:function(e){return!(e<this.resourceTypes.length)}}},p=m,h=(a("802f"),a("2877")),g=Object(h["a"])(p,n,r,!1,null,null,null);t["default"]=g.exports},"7db0":function(e,t,a){"use strict";var n=a("23e7"),r=a("b727").find,o=a("44d2"),i=a("ae40"),s="find",c=!0,u=i(s);s in[]&&Array(1)[s]((function(){c=!1})),n({target:"Array",proto:!0,forced:c||!u},{find:function(e){return r(this,e,arguments.length>1?arguments[1]:void 0)}}),o(s)},"802f":function(e,t,a){"use strict";a("e904")},a15b:function(e,t,a){"use strict";var n=a("23e7"),r=a("44ad"),o=a("fc6a"),i=a("a640"),s=[].join,c=r!=Object,u=i("join",",");n({target:"Array",proto:!0,forced:c||!u},{join:function(e){return s.call(o(this),void 0===e?",":e)}})},a434:function(e,t,a){"use strict";var n=a("23e7"),r=a("23cb"),o=a("a691"),i=a("50c4"),s=a("7b0b"),c=a("65f0"),u=a("8418"),l=a("1dde"),d=a("ae40"),f=l("splice"),m=d("splice",{ACCESSORS:!0,0:0,1:2}),p=Math.max,h=Math.min,g=9007199254740991,b="Maximum allowed length exceeded";n({target:"Array",proto:!0,forced:!f||!m},{splice:function(e,t){var a,n,l,d,f,m,v=s(this),y=i(v.length),x=r(e,y),O=arguments.length;if(0===O?a=n=0:1===O?(a=0,n=y-x):(a=O-2,n=h(p(o(t),0),y-x)),y+a-n>g)throw TypeError(b);for(l=c(v,n),d=0;d<n;d++)f=x+d,f in v&&u(l,d,v[f]);if(l.length=n,a<n){for(d=x;d<y-n;d++)f=d+n,m=d+a,f in v?v[m]=v[f]:delete v[m];for(d=y;d>y-n+a;d--)delete v[d-1]}else if(a>n)for(d=y-n;d>x;d--)f=d+n-1,m=d+a-1,f in v?v[m]=v[f]:delete v[m];for(d=0;d<a;d++)v[d+x]=arguments[d+2];return v.length=y-n+a,l}})},b85c:function(e,t,a){"use strict";a.d(t,"a",(function(){return r}));a("a4d3"),a("e01a"),a("d3b7"),a("d28b"),a("3ca3"),a("ddb0");var n=a("06c5");function r(e,t){var a="undefined"!==typeof Symbol&&e[Symbol.iterator]||e["@@iterator"];if(!a){if(Array.isArray(e)||(a=Object(n["a"])(e))||t&&e&&"number"===typeof e.length){a&&(e=a);var r=0,o=function(){};return{s:o,n:function(){return r>=e.length?{done:!0}:{done:!1,value:e[r++]}},e:function(e){throw e},f:o}}throw new TypeError("Invalid attempt to iterate non-iterable instance.\nIn order to be iterable, non-array objects must have a [Symbol.iterator]() method.")}var i,s=!0,c=!1;return{s:function(){a=a.call(e)},n:function(){var e=a.next();return s=e.done,e},e:function(e){c=!0,i=e},f:function(){try{s||null==a["return"]||a["return"]()}finally{if(c)throw i}}}}},c1f9:function(e,t,a){var n=a("23e7"),r=a("2266"),o=a("8418");n({target:"Object",stat:!0},{fromEntries:function(e){var t={};return r(e,(function(e,a){o(t,e,a)}),void 0,!0),t}})},c740:function(e,t,a){"use strict";var n=a("23e7"),r=a("b727").findIndex,o=a("44d2"),i=a("ae40"),s="findIndex",c=!0,u=i(s);s in[]&&Array(1)[s]((function(){c=!1})),n({target:"Array",proto:!0,forced:c||!u},{findIndex:function(e){return r(this,e,arguments.length>1?arguments[1]:void 0)}}),o(s)},d58f:function(e,t,a){var n=a("1c0b"),r=a("7b0b"),o=a("44ad"),i=a("50c4"),s=function(e){return function(t,a,s,c){n(a);var u=r(t),l=o(u),d=i(u.length),f=e?d-1:0,m=e?-1:1;if(s<2)while(1){if(f in l){c=l[f],f+=m;break}if(f+=m,e?f<0:d<=f)throw TypeError("Reduce of empty array with no initial value")}for(;e?f>=0:d>f;f+=m)f in l&&(c=a(c,l[f],f,u));return c}};e.exports={left:s(!1),right:s(!0)}},e904:function(e,t,a){}}]);