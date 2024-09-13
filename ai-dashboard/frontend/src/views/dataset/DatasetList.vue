<template>
  <div class="app-container">
    <div class="filter-container">
      <el-input
        v-model="listQuery.name"
        :placeholder="$t('dataset.name')"
        class="filter-item"
        style="width: 200px"
        @keyup.enter.native="handleFilter"
      />
      <el-button
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-search"
        @click="handleFilter"
      >
        {{ $t("dataset.search") }}
      </el-button>
      <el-button
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-edit"
        @click="handleCreate"
      >
        {{ $t("dataset.add") }}
      </el-button>
      <el-select
        v-model="curNamespace"
        :placeholder="$t('dataset.namespaceNotice')"
        style="margin-left: 10px"
        class="fr"
        type="primary"
        @change="handleChangeNamespace"
      >
        <el-option
          v-for="item in allNamespaces"
          :key="item.id"
          :label="item"
          :value="item"
        />
      </el-select>
    </div>

    <el-table
      :key="0"
      v-loading="listLoading"
      :data="list"
      border
      fit
      label-position="left"
      highlight-current-row
      style="width: 100%; margin-top: 20px"
    >
      <el-table-column align="center" label="ID">
        <template slot-scope="scope">
          {{ scope.$index + 1 }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('dataset.name')">
        <template slot-scope="{ row }">
          {{ row.name }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('dataset.namespace')">
        <template slot-scope="{ row }">
          {{ row.namespace }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('dataset.dataSource')">
        <template slot-scope="{ row }">
          <span v-html="row.mountPoints" />
        </template>
      </el-table-column>
      <el-table-column :label="$t('dataset.isAccelerate')">
        <template slot-scope="{ row }">
          <span>{{ row.mountPoints !== null ? "是" : "否" }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('dataset.status')">
        <template slot-scope="{ row }">
          <span>{{
            row.status.toLowerCase() === "bound" ? "Ready" : "NotReady"
          }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('dataset.createTime')">
        <template slot-scope="{ row }">
          <span>{{ row.createTime }}</span>
        </template>
      </el-table-column>
      <el-table-column
        :label="$t('dataset.operator')"
        align="center"
        class-name="small-padding fixed-width"
      >
        <template slot-scope="{ row, $index }">
          <el-button
            v-if="row.mountPoints === null"
            type="success"
            size="mini"
            @click="handleAcclerate(row)"
          >
            {{ $t("dataset.accelerate") }}
          </el-button>
          <el-button
            v-if="row.mountPoints !== null"
            type="danger"
            size="mini"
            @click="handleDelete(row, $index)"
          >
            {{ $t("dataset.removeAccelerate") }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <pagination
      v-show="list.length > 0"
      :total="list.length"
      :page.sync="listQuery.page"
      :limit.sync="listQuery.limit"
      @pagination="getList"
    />
    <el-dialog
      :title="textMap[dialogStatus]"
      :visible.sync="dialogFormVisible"
      width="1200px"
    >
      <el-form
        ref="dataForm"
        :rules="rules"
        :model="dataset"
        label-position="left"
        label-width="180px"
        style="margin-left: 50px"
      >
        <el-form-item :label="$t('dataset.name')" prop="name">
          <el-input
            v-model="dataset.name"
            :placeholder="$t('dataset.nameNotice')"
            resize="horizontal"
            @blur="syncRuntimeType()"
          />
        </el-form-item>
        <el-form-item label="Namespace" prop="namespace">
          <el-input
            v-model="curNamespace"
            :placeholder="$t('dataset.namespaceNotice')"
            :disabled="true"
            @change="filterByNamespace(curNamespace)"
          />
        </el-form-item>
        <el-form>
          <el-row>
            <el-col :span="20">{{"Dataset Conf"}}</el-col>
            <el-col :span="4">
              <el-switch
                v-model="isInYaml"
                active-color="#13ce66"
                inactive-color="gray"
                :active-text="$t('dataset.isInYaml')"
                style="margin-top: 10px; margin-right: 10px"
                @change="syncYamlDataset()"
              />
            </el-col>
          </el-row>
        </el-form>
        <el-card v-if="isInYaml===false" class="mountPoints">
          <div slot="header" class="clearfix">
            <el-row>
              <el-col :span="20">{{ $t("dataset.dataSource") }}</el-col>
              <el-col :span="1">
                <i
                  class="el-icon-circle-plus-outline"
                  style="color: green"
                  @click="appendMountPoint"
                />
              </el-col>
              <el-col :span="3">{{ $t("dataset.add") }}</el-col>
            </el-row>
          </div>
          <div
            v-for="(mountPoint, index) in dataset.mountPointList"
            :key="'mountPoint' + index"
            :rules="{
              name: [
                { required: true, message: '名称不能为空', trigger: 'blur' },
              ],
              uri: [
                { required: true, message: 'uri不能为空', trigger: 'blur' },
              ],
            }"
          >
            <el-card style="margin-bottom: 5px">
              <el-form-item
                :label="$t('dataset.dataSourceType')"
                label-width="100px"
              >
                <el-col :span="20">
                  <el-radio-group
                    v-model="mountPoint.sourceType"
                    @change="resetMountPoint(mountPoint)"
                  >
                    <el-radio
                      v-for="item in sourceTypes"
                      :key="item.id"
                      :name="item.name"
                      :label="item.value"
                      :value="item.value"
                      @change="syncRuntimeType()"
                    />
                  </el-radio-group>
                </el-col>
                <el-col :span="1">
                  <i
                    class="el-icon-circle-close"
                    style="color: red"
                    @click="removeMountPoint(index)"
                  />
                </el-col>
              </el-form-item>
              <el-form-item
                :gutter="1"
                type="flex"
                :label="$t('dataset.dataSource')"
                label-width="100px"
              >
                <el-col :span="10">
                  <el-form-item
                    :prop="'mountPointList.' + index + '.uri'"
                    :rules="{
                      required: true,
                      message: $t('dataset.dataSourceEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-select
                      v-if="mountPoint.sourceType === 'PVC'"
                      v-model="mountPoint.uri"
                      :span="11"
                      @change="syncDefaultMountPointName(mountPoint)"
                    >
                      <el-option
                        v-for="item in availablePvcList"
                        :key="item.id"
                        :label="'pvc://' + item.name"
                        :value="item.name"
                      />
                    </el-select>
                    <el-input v-else v-model="mountPoint.uri" :span="11" />
                  </el-form-item>
                </el-col>
                <el-col :span="2" style="margin-left: 10px">{{
                  $t("dataset.subDirectory")
                }}</el-col>
                <el-col :span="10">
                  <el-form-item
                    :prop="'mountPointList.' + index + '.name'"
                    :rules="{
                      required: true,
                      message: $t('dataset.subDirectoryEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-input
                      v-model="mountPoint.name"
                      :placeholder="$t('dataset.subDirectoryNotice')"
                    />
                  </el-form-item>
                </el-col>
              </el-form-item>
              <el-card
                v-if="mountPoint.sourceType === '其他'"
                style="margin-left: 10px"
              >
                <div slot="header" class="clearfix">
                  <el-row>
                    <el-col :span="20">{{ $t("dataset.accessConfig") }}</el-col>
                    <el-col :span="1">
                      <i
                        class="el-icon-circle-plus-outline"
                        style="color: green"
                        @click="appendMountPointOption(index)"
                      /></el-col>
                    <el-col :span="3">{{ $t("dataset.add") }}</el-col>
                  </el-row>
                </div>
                <div
                  v-for="(option, otherOptionsIndex) in mountPoint.otherOptions"
                  :key="'option' + otherOptionsIndex"
                  :rules="{}"
                >
                  <el-row type="flex" class="row-bg">
                    <el-col :span="5">
                      <el-form-item
                        :label-width="'60px'"
                        label="key"
                        :prop="'mountPointList.' + index + '.otherOptions.' + otherOptionsIndex + '.key' "
                        :rules="{
                          required: true,
                          message: 'key不能为空',
                          trigger: 'blur',
                        }"
                      >
                        <el-input v-model="option.key" style="width: 100%" />
                      </el-form-item>
                    </el-col>
                    <el-col
                      v-if="option.issecret === undefined || !option.issecret"
                      :span="15"
                      style="margin-left: 10px"
                    >
                      <el-form-item
                        label="value"
                        :label-width="'70px'"
                        :prop=" 'mountPointList.' + index + '.otherOptions.' + otherOptionsIndex + '.value' "
                        :rules="{
                          required: true,
                          message: 'value不能为空',
                          trigger: 'blur',
                        }"
                      >
                        <el-input v-model="option.value" style="width: 100%" />
                      </el-form-item>
                    </el-col>
                    <el-col
                      v-if="option.issecret"
                      :span="8"
                      style="margin-left: 10px"
                    >
                      <el-form-item
                        label="value"
                        :label-width="'70px'"
                        :prop=" 'mountPointList.' + index + '.otherOptions.' + otherOptionsIndex + '.fromSecret' "
                        :rules="{
                          required: true,
                          message: 'secret不能为空',
                          trigger: 'blur',
                        }"
                      >
                        <el-select
                          v-model="option.fromSecret"
                          :placeholder="$t('dataset.secretNotice')"
                          filterable
                        >
                          <el-option
                            v-for="item in availableSecretList"
                            :key="item.id"
                            :label="item.name"
                            :value="item.name"
                          />
                        </el-select>
                      </el-form-item>
                    </el-col>
                    <el-col v-if="option.issecret" :span="7">
                      <el-form-item
                        :label-width="'10px'"
                        :prop=" 'mountPointList.' + index + '.otherOptions.' + otherOptionsIndex + '.fromSecretKey' "
                        :rules="{
                          required: true,
                          message: 'secret key不能为空',
                          trigger: 'blur',
                        }"
                      >
                        <el-select
                          v-model="option.fromSecretKey"
                          :placeholder="$t('dataset.secretKeyNotice')"
                          filterable
                        >
                          <el-option
                            v-for="(item, optionIndex) in availableSecretMap[
                              option.fromSecret
                            ]"
                            :key="'item' + optionIndex"
                            :label="item"
                            :value="item"
                          />
                        </el-select>
                      </el-form-item>
                    </el-col>
                    <el-col :span="3">
                      <el-switch
                        v-model="option.issecret"
                        active-color="#13ce66"
                        inactive-color="gray"
                        :active-text="$t('dataset.encrypt')"
                        style="margin-top: 10px; margin-left: 10px"
                      />
                    </el-col>
                    <el-col :span="1">
                      <i
                        class="el-icon-delete"
                        style="color: red; margin-top: 12px"
                        @click="removeMountPointOption(index)"
                      />
                    </el-col>
                  </el-row>
                </div>
              </el-card>
              <el-form-item
                v-if="mountPoint.sourceType === 'OSS'"
                :label="$t('dataset.accessConfig')"
                label-width="100px"
              >
                <div
                  v-for="(option, OssOptionIndex) in mountPoint.ossOptions"
                  :key="'option' + OssOptionIndex"
                >
                  <el-row
                    type="flex"
                    class="row-bg"
                    :gutter="2"
                    style="margin-top: 20px"
                  >
                    <el-form-item
                      :label="
                        option.key.split('.')[option.key.split('.').length - 1]
                      "
                    />
                    <el-col
                      v-if="option.issecret === undefined || !option.issecret"
                      :span="20"
                    >
                      <el-form-item
                        label-width="'0px'"
                        :prop=" 'mountPointList.' + index + '.ossOptions.' + OssOptionIndex + '.value' "
                        :rules="{
                          required: true,
                          message:
                            option.key.split('.')[
                              option.key.split('.').length - 1
                            ] + '不能为空',
                          trigger: 'blur',
                        }"
                      >
                        <el-input v-model="option.value" />
                      </el-form-item>
                    </el-col>
                    <el-col v-if="option.issecret" :span="10">
                      <el-form-item
                        label-width="'0px'"
                        :prop=" 'mountPointList.' + index + '.ossOptions.' + OssOptionIndex + '.fromSecret' "
                        :rules="{
                          required: true,
                          message: 'secret不能为空',
                          trigger: 'blur',
                        }"
                      >
                        <el-select
                          v-model="option.fromSecret"
                          :placeholder="$t('dataset.secretNotice')"
                          filterable
                        >
                          <el-option
                            v-for="item in availableSecretList"
                            :key="item.id"
                            :label="item.name"
                            :value="item.name"
                          />
                        </el-select>
                      </el-form-item>
                    </el-col>
                    <el-col v-if="option.issecret" :span="10">
                      <el-form-item
                        label-width="'0px'"
                        :prop="'mountPointList.' + index + '.ossOptions.' + OssOptionIndex + '.fromSecretKey' "
                        :rules="{
                          required: true,
                          message: 'secret key不能为空',
                          trigger: 'blur',
                        }"
                      >
                        <el-select
                          v-model="option.fromSecretKey"
                          :placeholder="$t('dataset.secretKeyNotice')"
                          filterable
                        >
                          <el-option
                            v-for="(
                              item, secretOptionIndex
                            ) in availableSecretMap[option.fromSecret]"
                            :key="'item' + secretOptionIndex"
                            :label="item"
                            :value="item"
                          />
                        </el-select>
                      </el-form-item>
                    </el-col>
                    <el-col v-if="option.key !== 'endpoint'" :span="4">
                      <el-switch
                        v-model="option.issecret"
                        active-color="#13ce66"
                        inactive-color="gray"
                        :active-text="$t('dataset.encrypt')"
                        style="margin-left: 10px"
                      />
                    </el-col>
                  </el-row>
                </div>
              </el-form-item>
            </el-card>
          </div>
        </el-card>
        <el-card v-if="isInYaml===false" class="nodeAffinity">
          <div slot="header" class="clearfix">
            <el-row :gutter="1">
              <el-col :span="20">{{ $t("dataset.nodeAffinity") }}</el-col>
              <el-col :span="1">
                <i
                  class="el-icon-circle-plus-outline"
                  style="color: green"
                  @click="appendNodeAffinity"
                /></el-col>
              <el-col :span="3">{{ $t("dataset.add") }}</el-col>
            </el-row>
          </div>
          <el-card
            v-if="dataset.nodeAffinityList !== undefined && dataset.nodeAffinityList.length > 0"
            class="nodeAffinity"
          >
            <div slot="header" class="clearfix">
              <el-row :gutter="1" type="flex" class="row-bg">
                <el-col :span="10">{{ $t("dataset.labelName") }}</el-col>
                <el-col :span="4" style="margin-left: 10px">{{
                  $t("dataset.op")
                }}</el-col>
                <el-col :span="10" style="margin-left: 10px">{{
                  $t("dataset.labelValue")
                }}</el-col>
              </el-row>
            </div>
            <div
              v-for="(nodeAffinity, index) in dataset.nodeAffinityList"
              :key="'nodeAffinity' + index"
              class="nodeAffinity"
            >
              <el-row
                :gutter="1"
                type="flex"
                class="row-bg"
                style="margin-top: 5px"
              >
                <el-col :span="10">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'nodeAffinityList.' + index + '.label'"
                    :rules="{
                      required: true,
                      message: $t('dataset.labelNameEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-input v-model="nodeAffinity.label" />
                  </el-form-item>
                </el-col>
                <el-col :span="4" style="margin-left: 10px">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'nodeAffinityList.' + index + '.operator'"
                    :rules="{
                      required: true,
                      message: $t('dataset.labelOpEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-select v-model="nodeAffinity.operator">
                      <el-option
                        v-for="item in affinityOperatorTypes"
                        :key="item.id"
                        :label="item.name"
                        :value="item.name"
                      />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="9" style="margin-left: 10px">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'nodeAffinityList.' + index + '.values'"
                    :rules="{
                      required: true,
                      message: $t('dataset.labelValueEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-input
                      v-model="nodeAffinity.values"
                      :placeholder="$t('dataset.labelValueNotice')"
                    />
                  </el-form-item>
                </el-col>
                <el-col :span="1">
                  <i
                    class="el-icon-delete"
                    style="color: red; margin-top: 14px"
                    @click="removeNodeAffinity(index)"
                /></el-col>
              </el-row>
            </div>
          </el-card>
        </el-card>
        <el-card v-if="isInYaml===false" class="nodeAffinity">
          <div slot="header" class="clearfix">
            <el-row :gutter="1">
              <el-col :span="20">{{ $t("dataset.tolerations") }}</el-col>
              <el-col :span="1">
                <i
                  class="el-icon-circle-plus-outline"
                  style="color: green"
                  @click="appendToleration"
                /></el-col>
              <el-col :span="3">{{ $t("dataset.add") }}</el-col>
            </el-row>
          </div>
          <el-card
            v-if="dataset.tolerationList !== undefined && dataset.tolerationList.length > 0"
            class="nodeAffinity"
          >
            <div slot="header" class="clearfix">
              <el-row :gutter="1" type="flex" class="row-bg">
                <el-col :span="6">{{ $t("dataset.labelName") }}</el-col>
                <el-col :span="3" style="margin-left: 10px">{{ $t("dataset.op") }}</el-col>
                <el-col :span="7" style="margin-left: 10px">{{ $t("dataset.labelValue") }}</el-col>
                <el-col :span="3" style="margin-left: 10px">{{ $t("dataset.tolerationEffect") }}</el-col>
                <el-col :span="5" style="margin-left: 10px">{{ $t("dataset.tolerationSeconds") }}</el-col>
              </el-row>
            </div>
            <div
              v-for="(toleration, index) in dataset.tolerationList"
              :key="'toleration' + index"
              class="nodeAffinity"
            >
              <el-row
                :gutter="1"
                type="flex"
                class="row-bg"
                style="margin-top: 5px"
              >
                <el-col :span="6">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'tolerationList.' + index + '.label'"
                    :rules="{
                      required: true,
                      message: $t('dataset.labelNameEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-input v-model="toleration.label" />
                  </el-form-item>
                </el-col>
                <el-col :span="3" style="margin-left: 10px">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'tolerationList.' + index + '.operator'"
                    :rules="{
                      required: true,
                      message: $t('dataset.labelOpEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-select v-model="toleration.operator">
                      <el-option
                        v-for="item in tolerationOperatorTypes"
                        :key="item.id"
                        :label="item.name"
                        :value="item.name"
                      />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="7" style="margin-left: 10px">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'tolerationList.' + index + '.value'"
                    :rules="{
                      required: true,
                      message: $t('dataset.labelValueEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-input
                      v-model="toleration.value"
                    />
                  </el-form-item>
                </el-col>
                <el-col :span="3" style="margin-left: 10px">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'tolerationList.' + index + '.effect'"
                    :rules="{
                      required: true,
                      message: $t('dataset.effectEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-select v-model="toleration.effect">
                      <el-option
                        v-for="item in tolerationEffectTypes"
                        :key="item.id"
                        :label="item.name"
                        :value="item.name"
                      />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="4" style="margin-left: 10px">
                  <el-form-item
                    label-width="'0px'"
                    :prop="'tolerationList.' + index + '.tolerationSeconds'"
                    :rules="{
                      required: true,
                      message: $t('dataset.tolerationSecondsEmptyNotice'),
                      trigger: 'blur',
                    }"
                  >
                    <el-input
                      v-model="toleration.tolerationSeconds"
                      :placeholder="$t('dataset.tolerationSecondsNotice')"
                    />
                  </el-form-item>
                </el-col>
                <el-col :span="1">
                  <i
                    class="el-icon-delete"
                    style="color: red; margin-top: 14px"
                    @click="removeToleration(index)"
                /></el-col>
              </el-row>
            </div>
          </el-card>
           </el-card>
        <el-card v-if="isInYaml===true">
          <div slot="header" class="clearfix">
            <el-row>
              <el-col :span="20" style="margin-top: 20px; margin-left: 0px">{{
                $t("dataset.datasetConfig")
              }}</el-col>
            </el-row>
          </div>
          <div>
            <code-mirror-editor
              ref="datasetEditor"
              :cm-theme="cmTheme"
              :cm-mode="cmMode"
              :auto-format-json="autoFormatJson"
              :json-indentation="jsonIndentation"
              :editor-value="datasetEditorValue"
            />
          </div>
        </el-card>
        <el-card>
          <div slot="header" class="clearfix">
            <el-row>
              <el-col :span="20" style="margin-top: 20px; margin-left: 0px">{{
                $t("dataset.runtimeConfig")
              }}</el-col>
              <el-col :span="1" style="margin-top: 20px; margin-left: 0px">{{ $t("dataset.runtimeType") }}</el-col>
              <el-col :span="3">
                <el-select
                  v-model="dataset.runtimeType"
                  active-color="#13ce66"
                  inactive-color="gray"
                  :active-text="$t('dataset.usingJindo')"
                  style="margin-top: 10px; margin-left: 0px"
                  @change="syncRuntimeType()"
                >
                  <el-option
                    v-for="item in runtimeTypeList"
                    :key="item.id"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-col>
            </el-row>
          </div>
          <div>
            <code-mirror-editor
              ref="cmEditor"
              :cm-theme="cmTheme"
              :cm-mode="cmMode"
              :auto-format-json="autoFormatJson"
              :json-indentation="jsonIndentation"
              :editor-value="cmEditorValue"
            />
          </div>
        </el-card>
      </el-form>
      <div slot="footer" class="dialog-footer">
        <el-button @click="cancelDialog()">
          {{ $t("dataset.cancel") }}
        </el-button>
        <el-button type="primary" @click="createOrUpdateDataset(dataset)">
          {{ $t("dataset.save") }}
        </el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import jshint from "jshint";
import axios from "axios";
import yaml from "js-yaml";
window.JSHINT = jshint.JSHINT;

import { fetchDatasetList, deleteDataset, createDataset } from "@/api/dataset";
import { fetchPvcList, fetchSecretList, fetchNamespaceList } from "@/api/k8s";
import { parseTime, isEmpty, parseUri } from "@/utils";
import Pagination from "@/components/Pagination"; // secondary package based on el-pagination
import CodeMirrorEditor from "@/components/CodeMirrorEditor";
import { parse } from 'path-to-regexp';

export default {
  name: "DatasetList",
  components: {
    Pagination,
    CodeMirrorEditor,
  },
  filters: {
    statusFilter(status) {
      const statusMap = {
        draft: "gray",
        deleted: "danger",
      };
      return statusMap[status];
    },
    parseTime: parseTime,
  },
  data() {
    return {
      list: [],
      isInYaml: false,
      dataSetList: [],
      listLoading: true,
      listQuery: {
        limit: 20,
        page: 1,
        name: undefined,
        namespace: undefined,
      },
      mountPointTemplate: {
        sourceType: "PVC",
        uri: "",
        name: "",
        ossOptions: [
          {
            key: "fs.oss.endpoint",
            value: "",
            issecret: undefined,
            fromSecret: "",
            fromSecretKey: "",
          },
          {
            key: "fs.oss.accessKeyId",
            value: "",
            issecret: true,
            fromSecret: "",
            fromSecretKey: "",
          },
          {
            key: "fs.oss.accessKeySecret",
            value: "",
            issecret: true,
            fromSecret: "",
            fromSecretKey: "",
          },
        ],
        otherOptions: [],
      },
      curNamespace: "default",
      dataset: {
        name: '',
        namespace: '',
        mountPointList: [],
        nodeAffinityList: [],
        tolerationList: [],
        runtimeConf: '',
        runtimeType: 'jindo'
      },
      dialogFormVisible: false,
      dialogStatus: '',
      textMap: {
        update: this.$t("dataset.editDataset"),
        create: this.$t("dataset.createDataset"),
      },
      tolerationEffectTypes: [
        { name: 'NoSchedule' },
        { name: 'NoExecute' }
      ],
      tolerationOperatorTypes: [
        { name: 'Equal' },
        { name: 'Exists' }
      ],
      affinityOperatorTypes: [
        { name: 'In' },
        { name: 'notIn' },
        { name: 'Exists' },
        { name: 'DoesNotExist' },
        { name: 'Gt' },
        { name: 'Lt' }
      ],
      sourceTypes: [
        {
          name: 'pvc',
          value: 'PVC'
        },
        {
          name: 'oss',
          value: 'OSS'
        },
        {
          name: 'other',
          value: 'OTHER'
        }
      ],
      rules: {
        name: [
          {
            required: true,
            message: this.$t('dataset.nameEmptyNotice'),
            trigger: 'blur'
          }
        ],
        namespace: [
          {
            required: true,
            message: this.$t('dataset.namespaceEmptyNotice'),
            trigger: 'change'
          }
        ]
      },
      availablePvcList: [],
      allPvcList: undefined,
      allSecretList: undefined,
      availableSecretMap: {},
      availableSecretList: [],
      allNamespaces: undefined,
      runtimeTypeList: ["jindo", "alluxio"],
      defaultDatasetConf: [
        "apiVersion: data.fluid.io/v1alpha1",
        "kind: Dataset",
        "metadata:",
        "  name: defulat-dataset-name",
        "spec:",
        "  mounts:",
        "  - mountPoint:",
        "    name:",
        "  nodeAffinity:",
        "    required:",
        "      nodeSelectorTerms:",
        "        - matchExpressions:",
        "          - key: aliyun.accelerator/nvidia_name",
        "            operator: In",
        "            values:",
        "            - Tesla-V100-SXM2-16GB"
      ].join("\n"),

      defaultRuntimeConf: [
        "apiVersion: data.fluid.io/v1alpha1",
        "kind: AlluxioRuntime",
        "metadata:",
        "  name: default-runtime-name",
        "spec:",
        "  replicas: 4",
        "  data:",
        "    replicas: 1",
        "  tieredstore:",
        "    levels:",
        "      - mediumtype: SSD",
        "        path: /var/lib/docker/alluxio",
        "        quota: 150Gi",
        '        high: "0.99"',
        '        low: "0.8"',
      ].join("\n"),
      cmTheme: "default", // codeMirror主题
      // codeMirror主题选项
      cmThemeOptions: [
        "default",
        "3024-day",
        "3024-night",
        "abcdef",
        "ambiance",
        "ayu-dark",
        "ayu-mirage",
        "base16-dark",
        "base16-light",
        "bespin",
        "blackboard",
        "cobalt",
        "colorforth",
        "darcula",
        "dracula",
        "duotone-dark",
        "duotone-light",
        "eclipse",
        "elegant",
        "erlang-dark",
        "gruvbox-dark",
        "hopscotch",
        "icecoder",
        "idea",
        "isotope",
        "lesser-dark",
        "liquibyte",
        "lucario",
        "material",
        "material-darker",
        "material-palenight",
        "material-ocean",
        "mbo",
        "mdn-like",
        "midnight",
        "monokai",
        "moxer",
        "neat",
        "neo",
        "night",
        "nord",
        "oceanic-next",
        "panda-syntax",
        "paraiso-dark",
        "paraiso-light",
        "pastel-on-dark",
        "railscasts",
        "rubyblue",
        "seti",
        "shadowfox",
        "solarized dark",
        "solarized light",
        "the-matrix",
        "tomorrow-night-bright",
        "tomorrow-night-eighties",
        "ttcn",
        "twilight",
        "vibrant-ink",
        "xq-dark",
        "xq-light",
        "yeti",
        "yonce",
        "zenburn",
      ],
      datasetEditorValue: "",
      cmEditorValue: "",
      cmEditorMode: "yaml", // 编辑模式
      // 编辑模式选项
      cmEditorModeOptions: [
        "default",
        "json",
        "sql",
        "javascript",
        "css",
        "xml",
        "html",
        "yaml",
        "markdown",
        "python",
      ],
      cmMode: "yaml", // codeMirror模式
      jsonIndentation: 2, // json编辑模式下，json格式化缩进 支持字符或数字，最大不超过10，默认缩进2个空格
      autoFormatJson: true, // json编辑模式下，输入框失去焦点时是否自动格式化，true 开启， false 关闭
    };
  },
  created() {
    this.dataset.mountPointList.push(
      JSON.parse(JSON.stringify(this.mountPointTemplate))
    )
    this.getList()
    this.fecthNamespace()
    this.fetchSecret()
  },
  methods: {
    getList() {
      axios
        .all([this.fetchPVC(), this.fetchDataset()])
        .then(
          axios.spread((pvcResponse, datasetResponse) => {
            this.allPvcList = this.parsePvcList(pvcResponse)
            this.availablePvcList = [...this.allPvcList]
            this.dataSetList = this.parseDataset(datasetResponse)
            this.refreshTableList()
            this.filterPvcByNamespace(this.curNamespace)
            this.listLoading = false
          })
        )
        .catch((error) => {
          this.$notify({
            title: this.$t('dataset.retry'),
            message: this.$t('dataset.getDataFailed') + ',' + error,
            type: 'error',
            duration: 3000
          })
        })
        .finally(() => {
          this.listLoading = false
        })
    },
    syncRuntimeType() {
      if (this.dataset.mountPointList && this.dataset.mountPointList.length === 1) {
        if (this.dataset.mountPointList[0].sourceType === 'PVC') {
          this.dataset.runtimeType = 'alluxio'
          this.runtimeTypeList = ['alluxio']
          this.syncRuntimeConf(this.dataset.name, this.dataset.runtimeType)
          return
        }
        this.syncRuntimeConf(this.dataset.name, this.dataset.runtimeType)
        this.runtimeTypeList = ['jindo', 'alluxio']
        return
      }
      this.dataset.runtimeType = 'alluxio'
      this.syncRuntimeConf(this.dataset.name, this.dataset.runtimeType)
      this.runtimeTypeList = ['alluxio']
      return
    },
    filterByNamespace(namespace) {
      this.filterPvcByNamespace(namespace)
      this.filterSecretByNamespace(namespace)
      this.clearMountPoints()
    },
    clearMountPoints() {
      for (var i = 0; i < this.dataset.mountPointList.length; i++) {
        var mountPoint = this.dataset.mountPointList[i];
        if (mountPoint.sourceType.toLowerCase() === "pvc") {
          mountPoint.uri = "";
        }
        for (var j = 0; j < mountPoint.ossOptions.length; j++) {
          var ossOption = mountPoint.ossOptions[j];
          if (ossOption.issecret) {
            ossOption.fromSecret = "";
            ossOption.fromSecretKey = "";
          }
        }
        for (j = 0; j < mountPoint.otherOptions.length; j++) {
          var otherOption = mountPoint.otherOptions[j];
          if (otherOption.issecret) {
            otherOption.fromSecret = "";
            otherOption.fromSecretKey = "";
          }
        }
      }
    },
    filterSecretByNamespace(namespace) {
      this.availableSecretList = []
      this.availableSecretMap = {}
      if (this.allSecretList === null) {
        return
      }
      if (isEmpty(namespace)) {
        return
      }
      for (var i = 0; i < this.allSecretList.length; i++) {
        var item = this.allSecretList[i]
        if (item.namespace !== namespace) {
          continue
        }
        this.availableSecretMap[item.name] = item.keys === undefined ? [] : item.keys
        this.availableSecretList.push(item)
      }
    },
    filterPvcByNamespace(namespace) {
      this.availablePvcList = []
      if (this.allPvcList === null) {
        return
      }
      if (isEmpty(namespace)) {
        return
      }
      for (var i = 0; i < this.allPvcList.length; i++) {
        var pvc = this.allPvcList[i]
        if (pvc.namespace !== namespace) {
          continue
        }
        this.availablePvcList.push(pvc)
      }
    },
    syncRuntimeConf(datasetName, runtimeType) {
      var datasetConf
      if (this.$refs.datasetEditor) {
        datasetConf = this.$refs.datasetEditor.getValue()
      }
      var runtimeConf = this.$refs.cmEditor.getValue()
      if (isEmpty(runtimeConf)) {
        runtimeConf = this.defaultRuntimeConf
      }
      if (!isEmpty(datasetName)) {
        runtimeConf = this.syncRuntimeName(runtimeConf, datasetName)
        if (this.$refs.datasetEditor) {
          datasetConf = this.syncRuntimeName(datasetConf, datasetName)
        }
      }
      if (!isEmpty(runtimeType)) {
        runtimeConf = this.syncRuntimePath(runtimeConf, runtimeType)
      }
      this.$refs.cmEditor.setValue(runtimeConf)
      if (this.$refs.datasetEditor) {
        this.$refs.datasetEditor.setValue(datasetConf)
      }
    },
    syncRuntimePath(runtimeConf, runtimeType) {
      var data = yaml.loadAll(runtimeConf)
      if (isEmpty(runtimeType)) {
        return
      }
      if (data === null || data === undefined || data.length < 1) {
        return
      }
      data[0].kind = runtimeType === 'jindo' ? 'JindoRuntime' : 'AlluxioRuntime'
      var path = runtimeType === 'jindo' ? 'jindo' : 'alluxio'
      if (data[0].spec &&
        data[0].spec.tieredstore &&
        data[0].spec.tieredstore.levels &&
        !isEmpty(data[0].spec.tieredstore.levels)
      ) {
        data[0].spec.tieredstore.levels[0].path = '/var/lib/docker/' + path
      }
      var newValue = yaml.dump(data[0])
      return newValue
    },
    syncDatasetName(datasetConf, dataset) {
      var data = yaml.loadAll(datasetConf)
      if (isEmpty(data)) {
        return
      }
      data[0] = this.formatK8sDatasetFromUI(dataset, data[0])
      return yaml.dump(data[0])
    },
    syncRuntimeName(runtimeConf, datasetName) {
      var data = yaml.loadAll(runtimeConf)
      if (isEmpty(datasetName)) {
        return
      }
      if (data === null || data === undefined || data.length < 1) {
        return
      }
      data[0].metadata.name = datasetName
      var newValue = yaml.dump(data[0])
      return newValue
    },
    cancelDialog() {
      this.dialogFormVisible = false
      this.resetDataset()
    },
    appendToleration() {
      this.dataset.tolerationList.push({
        label: '',
        operator: '',
        value: '',
        effect: '',
        tolerationSeconds: 60
      })
    },
    removeToleration(index) {
      this.dataset.tolerationList.splice(index, 1)
    },
    appendNodeAffinity() {
      this.dataset.nodeAffinityList.push({
        label: '',
        operator: '',
        values: ''
      })
    },
    removeNodeAffinity(index) {
      this.dataset.nodeAffinityList.splice(index, 1);
    },
    appendMountPointOption(index) {
      var sourceType = this.dataset.mountPointList[index].sourceType;
      if (sourceType === "其他") {
        this.dataset.mountPointList[index].otherOptions.push({
          key: "",
          value: "",
          issecret: false,
          fromSecret: "",
          fromSecretKey: "",
        });
      }
    },
    syncDefaultMountPointName(mountPoint) {
      let defaultMountPointName = mountPoint.uri
      const { type, path } = parseUri(mountPoint.uri)
      defaultMountPointName = path
      mountPoint.name = defaultMountPointName + '-acc'
    },
    resetMountPoint(mountPoint) {
      mountPoint.name = "";
      mountPoint.uri = "";
      mountPoint.ossOptions = [
        {
          key: "fs.oss.endpoint",
          value: "",
          issecret: undefined,
          fromSecret: "",
          fromSecretKey: "",
        },
        {
          key: "fs.oss.accessKeyId",
          value: "",
          issecret: true,
          fromSecret: "",
          fromSecretKey: "",
        },
        {
          key: "fs.oss.accessKeySecret",
          value: "",
          issecret: true,
          fromSecret: "",
          fromSecretKey: "",
        },
      ];
      mountPoint.otherOptions = [];
    },
    removeMountPointOption(index) {
      var mountPointList = this.dataset.mountPointList[index];
      var sourceType = this.dataset.mountPointList[index].sourceType;
      if (sourceType === "其他") {
        mountPointList.otherOptions.splice(index, 1);
      }
    },
    parsePvcList(response) {
      var parsedPvcList = [];
      for (var i = 0; i < response.data.length; i++) {
        parsedPvcList.push(response.data[i]);
      }
      return parsedPvcList;
    },
    fetchPVC() {
      var pvcQuery = {
        limit: 20,
        page: 1,
        name: this.listQuery.name,
        namespace: this.curNamespace,
      };
      return fetchPvcList(pvcQuery);
    },
    fecthNamespace() {
      fetchNamespaceList()
        .then((response) => {
          if (response === undefined || response.data === undefined) {
            return;
          }
          this.allNamespaces = response.data.map((x) => x.name);
        })
        .catch((error) => {
          this.$notify({
            title: this.$t("dataset.retry"),
            message: this.$t("dataset.getNamespaceFailed") + "," + error,
            type: "error",
            duration: 2000,
          });
        });
    },
    fetchSecret() {
      this.allSecretList = []
      var query = {
        limit: 20,
        page: 1,
        namespace: this.curNamespace
      }
      fetchSecretList(query)
        .then((response) => {
          for (var i = 0; i < response.data.length; i++) {
            this.allSecretList.push(response.data[i])
          }
          this.availableSecretList = [...this.allSecretList]
          this.filterSecretByNamespace(this.curNamespace)
        })
        .catch(() => {})
    },
    resetDataset() {
      this.listQuery.name = undefined
      this.listQuery.namespace = this.curNamespace
      if (this.allSecretList === undefined || this.allSecretList.length < 1) {
        this.fetchSecret()
      }
      if (this.allNamespaces === undefined || this.allNamespaces.length < 1) {
        this.fecthNamespace()
      }
      if (this.$refs.datasetEditor !== undefined) {
        this.$refs.datasetEditor.setValue(this.defaultDatasetConf)
      }
      this.dataset = {
        name: '',
        namespace: this.curNamespace,
        mountPointList: [JSON.parse(JSON.stringify(this.mountPointTemplate))],
        nodeAffinityList: [],
        tolerationList: [],
        runtimeConf: '',
        runtimeType: 'jindo',
        datasetConf: ''
      }
      this.syncRuntimeType()
    },
    handleChangeNamespace() {
      this.getList()
    },
    handleAcclerate(row) {
      this.dialogStatus = 'update'
      this.dataset.mountPointList[0].uri = row.name
      this.dataset.mountPointList[0].name = row.name + '-acc'
      this.dataset.namespace = row.namespace
      this.dataset.name = row.name + '-acc'
      this.dataset.runtimeType = 'alluxio'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.syncRuntimeType()
        this.$refs['dataForm'].clearValidate()
      })
    },
    handleFilter() {
      this.getList()
    },
    parseDatasetItem(item) {
      var parsedDataset = {}
      var metadata = item.metadata
      var status = item.status
      var spec = item.spec
      parsedDataset.namespace = metadata.namespace
      parsedDataset.name = metadata.name
      if (!isEmpty(status)) {
        parsedDataset.status = status.phase
      }
      parsedDataset.nodeAffinity = spec.nodeAffinity || []
      parsedDataset.tolerations = spec.tolerations || []
      var mountPointPathList = []
      for (var mpIdx = 0; mpIdx < spec.mounts.length; mpIdx++) {
        var mountPoint = spec.mounts[mpIdx]
        mountPointPathList.push(mountPoint.mountPoint)
      }
      parsedDataset.mountPoints = mountPointPathList
        .map((x) => x + '<br>')
        .join('')
      parsedDataset.origMountPointList = spec.mounts
      parsedDataset.createTime = metadata.creationTimestamp
      if (!isEmpty(metadata.managedFields)) {
        parsedDataset.updateTime = metadata.managedFields[metadata.managedFields.length - 1].time
      }
      return parsedDataset
    },
    parseDataset(response) {
      var parsedDatasetList = []
      var datasetItems = response.data
      for (var i = 0; i < datasetItems.length; i++) {
        var parsedDataset = this.parseDatasetItem(datasetItems[i])
        parsedDatasetList.push(parsedDataset)
      }
      return parsedDatasetList
    },
    fetchDataset() {
      this.listQuery.namespace = this.curNamespace;
      return fetchDatasetList(this.listQuery);
    },
    handleCreate() {
      this.dialogStatus = 'create'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.resetDataset()
        this.$refs['dataForm'].clearValidate()
        this.listLoading = false
      })
    },
    appendMountPoint() {
      this.dataset.mountPointList.push(
        JSON.parse(JSON.stringify(this.mountPointTemplate))
      )
      this.syncRuntimeType()
    },
    removeMountPoint(index) {
      this.dataset.mountPointList.splice(index, 1)
      this.syncRuntimeType()
    },
    handleUpdate(row) {
      this.dataset = Object.assign({}, row); // copy obj
      this.dialogStatus = "update";
      this.dialogFormVisible = true;
      this.$nextTick(() => {
        this.$refs["dataForm"].clearValidate();
        this.listLoading = false;
      });
    },
    handleDelete(row, index) {
      const tempData = Object.assign({}, row)
      deleteDataset(tempData)
        .then(() => {
          this.list.splice(index, 1)
          this.listLoading = true
          this.fetchDataset()
            .then((response) => {
              this.dataSetList = this.parseDataset(response)
              this.refreshTableList(row)
            })
            .catch(() => {
              this.$notify({
                title: this.$t('dataset.retry'),
                message: this.$t('dataset.getDataFailed'),
                type: 'error',
                duration: 2000
              })
            })
            .finally(() => {
              this.$notify({
                title: this.$t('dataset.success'),
                message: this.$t('dataset.removeAccelerateSuccess'),
                type: 'success',
                duration: 1000
              })
            })
        })
        .catch(() => {
          this.$notify({
            title: this.$t('dataset.retry'),
            message: this.$t('dataset.removeAccelerateFailed'),
            type: 'error',
            duration: 2000
          })
        })
        .finally(() => {
          this.listLoading = false
        })
    },
    formatUIDatasetFromK8s(dataset) {
      var uiDataset = {
        name: '',
        namespace: dataset.namespace,
        mountPointList: [],
        nodeAffinityList: [],
        tolerationList: [],
        runtimeConf: ''
      }
      var uiMountPointList = []
      for (var i = 0; i < dataset.origMountPointList.length; i++) {
        var origMountPoint = dataset.origMountPointList[i]
        if (origMountPoint.mountPoint) {
          var { type, path } = parseUri(origMountPoint.mountPoint)
        }
        type = type ? type.toUpperCase() : 'PVC'
        var uiMountPoint = {
          sourceType: type,
          uri: path,
          name: origMountPoint.name
        }
        var uiOptions = []
        var option = origMountPoint.options
        for (var k = 0; k < Object.keys(option).length; k++) {
          var curKey = Object.keys(option)[k]
          var uiOption = {
            key: curKey,
            value: option[curKey],
            fromSecret: undefined,
            fromSecretKey: undefined,
            issecret: undefined
          }
          uiOptions.push(uiOption)
        }

        for (var j = 0; j < (origMountPoint.encryptOptions || []).length; j++) {
          option = origMountPoint.encryptOptions[j]
          uiOption = {
            key: curKey,
            value: option[curKey],
            fromSecret: option.valueFrom.secretKeyRef.name,
            fromSecretKey: option.valueFrom.secretKeyRef.key,
            issecret: true
          }
          uiOptions.push(uiOption)
        }
        
        if (type === 'OSS') {
          uiMountPoint['ossOptions'] = uiOptions
        } else if (type !== 'PVC') {
          uiMountPoint['otherOptions'] = uiOptions
        }

        uiMountPointList.push(uiMountPoint)
      }
      // node affinity
      var uiNodeAffinitys = []
      if (dataset.nodeAffinity && dataset.nodeAffinity.required) {
        for (j = 0; j < (dataset.nodeAffinity.required.nodeSelectorTerms || []).length; j++) {
          var nf = dataset.nodeAffinity.required.nodeSelectorTerms[j].matchExpressions[0]
          uiNodeAffinitys.push({
            label: nf.key,
            operator: nf.operator,
            values: (nf.values || []).join(';')
          })
        }
      }
      // tolerations
      var uiTolerations = []
      for (j = 0; j < (dataset.tolerations || []).length; j++) {
        var toleration = dataset.tolerations[j]
        uiTolerations.push({
          key: toleration.key,
          operator: toleration.operator,
          value: toleration.value,
          effect: toleration.effect,
          tolerationSeconds: !Number.isInteger(toleration.tolerationSeconds)?toleration.tolerationSeconds.parseInt():toleration.tolerationSeconds
        })
      }
      uiDataset.mountPointList = uiMountPointList || []
      uiDataset.nodeAffinityList = uiNodeAffinitys || []
      uiDataset.tolerationList = uiTolerations || []
      return uiDataset
    },
    formatK8sDatasetFromUI(dataset, originK8sDataset) {
      var resDataset = {
        apiVersion: 'data.fluid.io/v1alpha1',
        kind: 'Dataset',
        metadata: {
          name: dataset.name,
          namespace: dataset.namespace
        },
        spec: originK8sDataset.spec || []
      }
      var queryMountPoints = []
      if (
        dataset.mountPointList !== null &&
        dataset.mountPointList !== undefined
      ) {
        for (var i = 0; i < dataset.mountPointList.length; i++) {
          var queryMountPoint = {}
          var mountPoint = dataset.mountPointList[i]
          var sourceType = mountPoint.sourceType
          queryMountPoint.mountPoint = mountPoint.uri
          if (sourceType.toLowerCase() === 'pvc') {
            queryMountPoint.mountPoint = 'pvc://' + mountPoint.uri
          }
          if (sourceType.toLowerCase() === 'oss' && mountPoint.uri.toLowerCase().indexOf('oss://') < 0) {
            queryMountPoint.mountPoint = 'oss://' + mountPoint.uri
          }
          queryMountPoint.name = mountPoint.name

          var fromMountPoint = []
          if (mountPoint.sourceType === 'OSS') {
            fromMountPoint = mountPoint.ossOptions
          } else if (mountPoint.sourceType !== 'PVC') {
            fromMountPoint = mountPoint.otherOptions
          }
          var options = {}
          var encryptOptions = []
          for (var j = 0; j < fromMountPoint.length; j++) {
            const mp = fromMountPoint[j]
            if (!mp.issecret && mp.key == "fs.oss.endpoint") {
              options[mp.key] = mp.value
              continue
            }
            var eop = {
              name: mp.key,
              valueFrom: {
                secretKeyRef: {
                  name: mp.fromSecret,
                  key: mp.fromSecretKey
                }
              }
            }
            encryptOptions.push(eop)
          }
          if (mountPoint.sourceType !== 'PVC') {
            if (!isEmpty(options)) {
              queryMountPoint.options = options
            }
            if (!isEmpty(encryptOptions)) {
              queryMountPoint.encryptOptions = encryptOptions
            }
          }
          queryMountPoints.push(queryMountPoint)
        }
        resDataset.spec.mounts = queryMountPoints
      }
      // node affinitys
      if (!isEmpty(dataset.nodeAffinityList)) {
        resDataset.spec.nodeAffinity = {
          required: {
            nodeSelectorTerms: []
          }
        }
        var queryNodeAffinitys = []
        for (j = 0; j < dataset.nodeAffinityList.length; j++) {
          var nodeAffinity = dataset.nodeAffinityList[j]
          var queryNodeAffinity = {
            key: nodeAffinity.label,
            operator: nodeAffinity.operator,
            values: nodeAffinity.values.split(';')
          }
          queryNodeAffinitys.push(queryNodeAffinity)
        }
        resDataset.spec.nodeAffinity.required.nodeSelectorTerms.push({ matchExpressions: queryNodeAffinitys })
      } else {
        resDataset.spec.nodeAffinity = undefined
      }
      // tolerations
      if (!isEmpty(dataset.tolerationList)) {
        var tolerations = []
        for (j = 0; j < dataset.tolerationList.length; j++) {
          var uiToleration = dataset.tolerationList[j]
          var toleration = {
            effect: uiToleration.effect,
            key: uiToleration.label,
            value: uiToleration.value,
            operator: uiToleration.operator,
            tolerationSeconds: uiToleration.tolerationSeconds
          }
          tolerations.push(toleration)
        }
        resDataset.spec.tolerations = tolerations
      }
      return resDataset
    },
    refreshTableList(rowToIgnore) {
      this.list = []
      var datasetStringIds = new Map()
      for (var j = 0; j < this.dataSetList.length; j++) {
        var dataset = this.dataSetList[j]
        datasetStringIds.set(dataset.name + '.' + dataset.namespace, 1)
        for (var k = 0; k < dataset.origMountPointList.length; k++) {
          const mountPointUri = dataset.origMountPointList[k].mountPoint
          var { type, path } = parseUri(mountPointUri)
          if (type.toLowerCase() !== 'pvc') {
            continue
          }
          datasetStringIds.set(path + '.' + dataset.namespace, 1)
        }
      }
      if (rowToIgnore) {
        datasetStringIds.set(rowToIgnore.name + '.' + rowToIgnore.namespace, 1)
      }

      for (var i = 0; i < this.allPvcList.length; i++) {
        var item = this.allPvcList[i]
        item.nodeAffinity = null
        item.mountPoints = null
        var datasetStringId = item.name + '.' + item.namespace
        if (datasetStringIds.has(datasetStringId)) {
          continue
        }
        this.list.push(item)
      }
      for (j = 0; j < this.dataSetList.length; j++) {
        dataset = this.dataSetList[j]
        this.list.push(dataset)
      }
    },
    syncYamlToDataset(datasetConf) {
      if (isEmpty(datasetConf)) {
        return
      }
      var data = yaml.loadAll(datasetConf)
      if (isEmpty(data[0])) {
        return
      }
      const parsedDatasetItem = this.parseDatasetItem(data[0])
      this.dataset = this.formatUIDatasetFromK8s(parsedDatasetItem)
      this.dataset.datasetConf = datasetConf
      this.syncRuntimeType()
      this.$refs['dataForm'].clearValidate()
    },
    syncYamlDataset() {
      this.$refs['dataForm'].clearValidate()
      if (this.isInYaml) {
        var datasetConf = this.defaultDatasetConf
        if (this.dataset.datasetConf) {
          datasetConf = this.dataset.datasetConf
        }
        datasetConf = this.syncDatasetName(
          datasetConf,
          this.dataset
        )
        this.$nextTick(() => {
          this.$refs['dataForm'].clearValidate()
          this.$refs.datasetEditor.setValue(datasetConf)
        })
      } else {
        const datasetConf = this.$refs.datasetEditor.getValue()
        this.$nextTick(() => {
          this.$refs['dataForm'].clearValidate()
          this.syncYamlToDataset(datasetConf)
        })
      }
    },
    toK8sDataset() {
      var resDataset = {}
      this.dataset.datasetConf = this.$refs.datasetEditor === undefined ? undefined : this.$refs.datasetEditor.getValue()
      if (!this.dataset.datasetConf) {
        const datasetConf = this.syncDatasetName(
          this.defaultDatasetConf,
          this.dataset
        )
        this.dataset.datasetConf = datasetConf
      }
      resDataset['datasetConf'] = this.dataset.datasetConf
      resDataset['runtimeConf'] = this.$refs.cmEditor.getValue()
      return resDataset
    },
    createOrUpdateDataset(dataset) {
      var k8sdataset = this.toK8sDataset(dataset)
      this.$refs['dataForm'].validate((valid) => {
        if (!valid) {
          return
        }
        if (!k8sdataset.datasetConf) {
          this.$notify({
            title: this.$t('dataset.warning'),
            message: this.$t('dataset.datasetConfigEmpty'),
            type: 'warning'
          })
          return
        }
        if (!k8sdataset.runtimeConf) {
          this.$notify({
            title: this.$t('dataset.warning'),
            message: this.$t('dataset.runtimeConfigEmpty'),
            type: 'warning'
          })
          return
        }
        createDataset(k8sdataset)
          .then((response) => {
            this.dialogFormVisible = false
            this.listLoading = true
            this.resetDataset()
            this.fetchDataset()
              .then((response) => {
                this.dataSetList = this.parseDataset(response)
                this.refreshTableList()
                this.$notify({
                  title: this.$t('dataset.success'),
                  message: this.$t('dataset.executeSuccess'),
                  type: 'success',
                  duration: 2000
                })
              })
              .catch(() => {
                this.$notify({
                  title: this.$t('dataset.retry'),
                  message: this.$t('dataset.getDataFailed'),
                  type: 'error',
                  duration: 2000
                })
              })
          })
          .catch((error) => {
            this.$notify({
              title: this.$t('dataset.refresh'),
              message: this.$t('dataset.accelerateFailed') + ',' + error,
              type: 'error',
              duration: 3000
            })
          })
          .finally(() => {
            this.listLoading = false
          })
      })
    },
    fillDefaultAcceleratedName() {
      if (this.dataset.acceleratedName === "") {
        this.dataset.acceleratedName = this.dataset.name + "-acc";
      }
    },

    // 切换编辑模式事件处理函数
    onEditorModeChange(value) {
      switch (value) {
        case "json":
          this.cmMode = "application/json";
          break;
        case "sql":
          this.cmMode = "sql";
          break;
        case "javascript":
          this.cmMode = "javascript";
          break;
        case "xml":
          this.cmMode = "xml";
          break;
        case "css":
          this.cmMode = "css";
          break;
        case "html":
          this.cmMode = "htmlmixed";
          break;
        case "yaml":
          this.cmMode = "yaml";
          break;
        case "markdown":
          this.cmMode = "markdown";
          break;
        case "python":
          this.cmMode = "python";
          break;
        default:
          this.cmMode = "application/json";
      }
    },
  },
};
</script>

<style lang="scss" scoped>
.mountPoints {
  //display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 5px;
  margin-bottom: 5px;
  //height: 100vh;
}
.nodeAffinity {
  //display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 5px;
  margin-bottom: 5px;
  //height: 100vh;
}
.el-select {
  display: block;
  //  padding-right: 8px;
}
</style>
