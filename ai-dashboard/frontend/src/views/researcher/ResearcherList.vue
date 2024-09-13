<template>
  <div class="app-container">
    <div class="filter-container">
      <el-input
        v-model="listQuery.userName"
        :placeholder="$t('user.userName')"
        style="width: 200px"
        class="filter-item"
        @keyup.enter.native="handleFilter"
      />
      <el-button
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-search"
        @click="handleFilter"
      >
        {{ $t('user.search') }}
      </el-button>
      <el-button
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-edit"
        @click="handleCreate"
      >
        {{ $t('user.add') }}
      </el-button>
      <el-button
        class="fr"
        style="margin-left: 10px"
        type="default"
        icon="el-icon-refresh-left"
        @click="refresh"
      >
        {{ $t('userGroup.refresh') }}
      </el-button>
    </div>

    <el-table
      v-loading="listLoading"
      :data="list"
      element-loading-text="Loading"
      border
      fit
      highlight-current-row
      style="margin-top: 20px"
    >
      <el-table-column align="center" label="ID">
        <template slot-scope="scope">
          {{ scope.$index + 1 }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('user.userName')">
        <template slot-scope="{ row }">
          {{ row.userName }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('user.groupName')">
        <template slot-scope="{ row }">
          <div v-for="n in row.groups" :key="n" class="mr2">{{ n }}</div>
        </template>
      </el-table-column>
      <el-table-column :label="$t('user.userType')">
        <template slot-scope="{ row }">
          <div v-for="n in row.apiRoles" :key="n" class="mr2">{{ n }}</div>
        </template>
      </el-table-column>
      <el-table-column :label="$t('user.roles')" width="180">
        <template slot-scope="scope">
          <div v-for="n in scope.row.roles" :key="n.namespace">
            {{ n.namespace }}: {{ (n.roleNames || []).join(',') }}
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="$t('user.clusterRoles')">
        <template slot-scope="scope">
          <div v-for="n in scope.row.clusterRoles" :key="n">{{ n }}</div>
        </template>
      </el-table-column>
      <el-table-column class-name="status-col" :label="$t('user.kubeConfig')">
        <template slot-scope="scope">
          <el-link type="primary" @click="downloadKubeConfig(scope.row)">{{ $t("user.download") }}</el-link>
        </template>
      </el-table-column>
      <el-table-column class-name="status-col" :label="$t('user.bearerToken')">
        <template slot-scope="scope">
          <el-link type="primary" @click="copyBearerToken(scope.row)">{{
            $t("user.copyToken")
          }}</el-link>
        </template>
      </el-table-column>
      <el-table-column :label="$t('user.createTime')">
        <template slot-scope="scope">
          <span>{{
            scope.row.createTime | parseTime("{y}-{m}-{d} {h}:{i}")
          }}</span>
        </template>
      </el-table-column>
      <el-table-column
        :label="$t('user.operator')"
        align="center"
        width="160"
        class-name="small-padding fixed-width"
      >
        <template slot-scope="scope">
          <el-button
            type="primary"
            size="mini"
            @click="handleUpdate(scope.row)"
          >
            {{ $t('user.edit') }}
          </el-button>
          <el-button
            size="mini"
            type="danger"
            @click="handleDelete(scope.row, scope.$index)"
          >
            {{ $t('user.delete') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <pagination
      v-show="list.length > 0"
      :total="list.length"
      :page.sync="listQuery.page"
      :limit.sync="listQuery.limit"
      @pagination="fetchData"
    />
    <el-dialog :title="textMap[dialogStatus]" :visible.sync="dialogFormVisible">
      <el-form
        ref="dataForm"
        :model="researcher"
        label-position="left"
        label-width="120px"
        style="margin-left: 20px"
      >
        <el-form-item :label="$t('user.userName')" :rules="{ required: true, message: $t('user.userNameEmptyNotice'), trigger: 'blur' }">
          <el-input v-if="dialogStatus==='update'" v-model="researcher.userName" />
          <el-select
            v-else
            v-model="researcher.userName"
            :placeholder="$t('user.userNameNotice')"
            filterable
            allow-create
            @change="syncUserId(researcher)"
          >
            <el-option
              v-for="item in userNameList"
              :key="item.userId"
              :label="item.userName"
              :value="item.userName"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('user.userType')" prop="apiRoles" :rules="{ required: true, message: $t('user.userTypeEmptyNotice'), trigger: 'blur' }">
          <el-select
            v-model="researcher.apiRoles[0]"
            :placeholder="$t('user.userTypeNotice')"
          >
            <el-option
              v-for="item in apiRoleList"
              :key="item.id"
              :label="item"
              :value="item"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('user.groupName')" prop="groups" :rules="{required: true, validator:userGroupValidator, message: $t('user.groupEmptyNotice'), trigger: 'blur'}">
          <el-select
            v-model="researcher.groups"
            :placeholder="$t('user.clusterRoleNotice')"
            multiple
            @change="sycnRoleNamespaceByGroup(researcher)"
          >
            <el-option
              v-for="item in userGroupList"
              :key="item.metadata.uid"
              :label="item.spec.groupName"
              :value="item.spec.groupName"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('user.quota')">
          <el-select v-model="researcher.roleNamespaces" multiple>
            <el-option
              v-for="item in filterNamespaceByGroup(researcher)"
              :key="item.id"
              :label="item"
              :value="item"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <div slot="footer" class="dialog-footer">
        <el-button @click="cancelEdit()"> {{ $t('user.cancel') }} </el-button>
        <el-button
          type="primary"
          @click="dialogStatus === 'create' ? createData() : updateData()"
        >
          {{ $t('user.save') }}
        </el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>

import axios from 'axios'
import {
  createResearcher,
  fetchRamUserList,
  fetchResearcherList,
  deserializeK8sUser,
  updateResearcher,
  deleteResearcher,
  downloadK8sConfig,
  getBearerTokenByUser
} from '@/api/researcher'
import { fetchUserGroup, fetchUserGroupNamespaces } from '@/api/userGroup'
import { parseTime, isEmpty } from '@/utils'
import Pagination from '@/components/Pagination' // secondary package based on el-pagination

export const USER_TYPE_ADMIN = 'admin'
export const USER_TYPE_RESEARCHER = 'researcher'
export const DEFAULT_QUOTA_NAMESPACE = 'default-group'
export const DEFAULT_USER_GROUP_SPEC_NAME = 'defaultUserGroup'

export const researcherTemplate = {
  id: undefined,
  uid: '',
  userName: '',
  apiRoles: [USER_TYPE_RESEARCHER],
  groups: [DEFAULT_USER_GROUP_SPEC_NAME],
  aliuid: '',
  namespace: undefined,
  kubeConfig: '',
  status: '',
  roleNamespaces: [DEFAULT_QUOTA_NAMESPACE]
}

export default {
  name: 'ResearcherList',
  components: {
    Pagination
  },
  filters: {
    parseTime: parseTime
  },
  data() {
    return {
      userGroupValidator: (rule, value, callback) => {
        if (isEmpty(value)) {
          callback(new Error(this.$t('user.groupEmptyNotice')))
          return
        }
        callback()
      },
      roleNamesValidate: (rule, value, callback) => {
        if (isEmpty(value)) {
          callback(new Error(this.$t('user.roleEmptyNotice')))
          return
        }
        callback()
      },
      list: [],
      listLoading: true,
      listQuery: {
        page: 1,
        limit: 20,
        userName: undefined
      },
      researcher: JSON.parse(JSON.stringify(researcherTemplate)),
      dialogFormVisible: false,
      dialogStatus: '',
      textMap: {
        update: this.$t('user.editUser'),
        create: this.$t('user.addUser')
      },
      groupNamespaces: {},
      userNameList: [],
      ramUserList: [],
      userGroupList: [],
      apiRoleList: [USER_TYPE_ADMIN, USER_TYPE_RESEARCHER]
    }
  },
  created() {
    console.log('researcher created')
    this.fetchData()
    this.fetchQuotaNamespaces()
    this.fetchUserNameList()
    this.fetchUserGroupList()
  },
  methods: {
    fetchData() {
      this.listLoading = true
      axios.all([
        fetchResearcherList(this.listQuery),
        fetchUserGroup({ page: 1, limit: 1000000 })
      ]).then(axios.spread((reseacherListResponse, userGroupResponse) => {
        if (!isEmpty(reseacherListResponse.data.items)) {
          this.userGroupList = userGroupResponse.data.items || []
          const nameMap = new Map(this.userGroupList.map(x => [x.metadata.name, x.spec.groupName]))
          this.list = reseacherListResponse.data.items.map(x => deserializeK8sUser(x)).map(x => this.groupNameToSpecName(x, nameMap)).map(x => this.addRoleNamespaces(x))
          this.refreshUserNameList(this.ramUserList, this.list)
        }
      })).catch((error) => {
        this.$notify({
          title: this.$t('user.retry'),
          message: this.$t('user.getUserFailed') + ',' + error,
          type: 'error',
          duration: 3000
        })
      }).finally(() => {
        this.listLoading = false
      })
    },
    refresh() {
      this.listQuery.userName = undefined
      this.fetchData()
    },
    sycnRoleNamespaceByGroup(researcher) {
      if (!isEmpty(this.groupNamespaces)) {
        researcher.roleNamespaces = researcher.groups.map(x => isEmpty(this.groupNamespaces[x]) ? [] : [...this.groupNamespaces[x]]).flat()
      }
    },
    filterNamespaceByGroup(researcher) {
      var res = []
      if (!isEmpty(this.groupNamespaces)) {
        var newNamespace = researcher.groups.map(x => isEmpty(this.groupNamespaces[x]) ? [] : [...this.groupNamespaces[x]]).flat()
        res = res.concat(newNamespace)
      }
      return res.filter((v, i, self) => self.indexOf(v) === i)
    },
    fetchQuotaNamespaces() {
      fetchUserGroupNamespaces().then(response => {
        if (response && response.code === 10000) {
          this.groupNamespaces = response.data
        }
      }).finally(() => {
        this.listLoading = false
      })
    },
    fetchUserNameList() {
      fetchRamUserList().then((response) => {
        this.userNameList = response.data
      }).catch((error) => {
        console.log('fetch user name list error', error)
        this.userNameList = []
      })
    },
    fetchUserGroupList(page = 1, limit = 100000) {
      fetchUserGroup({ page, limit }).then((response) => {
        this.userGroupList = response.data.items || []
      })
    },
    resetListQuery() {
      this.listQuery.userName = undefined
    },
    findAliuidByName(userName) {
      var ramUserNameMap = new Map(this.ramUserList.map(x => [x.userName, x.userId]))
      return ramUserNameMap.get(userName)
    },
    syncUserId(researcher) {
      this.researcher.aliuid = this.findAliuidByName(researcher.userName)
      this.refreshApiRoles(researcher)
      if (isEmpty(this.researcher.aliuid)) {
        this.researcher.apiRoles = [USER_TYPE_RESEARCHER]
      }
    },
    cancelEdit() {
      this.fetchData()
      this.dialogFormVisible = false
    },
    handleFilter() {
      this.listLoading = true
      this.fetchData()
    },
    refreshUserNameList(ramUserList, researcherList) {
      var researcherNameList = researcherList.map(x => x.userName)
      this.userNameList = ramUserList.filter(x => researcherNameList.indexOf(x.userName) < 1)
    },
    addRoleNamespaces(x) {
      if (x && x.roles) {
        x.roleNamespaces = x.roles.map(x => x.namespace)
      }
      return x
    },
    groupNameToSpecName(x, groupMaps) {
      x.groups = x.groups.map(x => groupMaps.get(x))
      return x
    },
    getUserNameList() {
      axios.all([
        fetchRamUserList(),
        fetchResearcherList(this.listQuery),
        fetchUserGroup({ page: 1, limit: 1000000 })
      ]).then(axios.spread((ramUserListResponse, reseacherListResponse, userGroupResponse) => {
        this.ramUserList = ramUserListResponse.data
        this.userGroupList = userGroupResponse.data.items || []
        if (!isEmpty(reseacherListResponse.data.items) && !isEmpty(this.userGroupList)) {
          const nameMap = new Map(this.userGroupList.map(x => [x.metadata.name, x.spec.groupName]))
          this.list = reseacherListResponse.data.items.map(x => deserializeK8sUser(x)).map(x => this.groupNameToSpecName(x, nameMap)).map(x => this.addRoleNamespaces(x))
          this.refreshUserNameList(this.ramUserList, this.list)
        }
      })).catch((error) => {
        this.$notify({
          title: this.$t('user.retry'),
          message: this.$t('user.getDataFailed') + ',' + error,
          type: 'error',
          duration: 3000
        })
      })
    },
    handleCreate() {
      this.researcher = JSON.parse(JSON.stringify(researcherTemplate))
      this.sycnRoleNamespaceByGroup(this.researcher)
      this.refreshApiRoles()
      this.dialogStatus = 'create'
      this.dialogFormVisible = true
      this.refreshUserList()
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    refreshUserList() {
      this.resetListQuery()
      if (isEmpty(this.groupNamespaces)) {
        this.fetchQuotaNamespaces()
      }
      if ((this.ramUserList === undefined || this.ramUserList.length < 1) && (this.list === undefined || this.list.length < 1)) {
        this.getUserNameList()
      } else if (this.ramUserList === undefined || this.ramUserList.length < 1) {
        fetchRamUserList().then(response => {
          if (response.data !== undefined) {
            this.ramUserList = response.data
            this.refreshUserNameList(this.ramUserList, this.list)
          }
        }).catch(() => {
          this.userNameList = []
        })
      } else if (this.list === undefined || this.list.length < 1) {
        this.fetchData()
      } else {
        this.refreshUserNameList(this.ramUserList, this.list)
      }
    },
    refreshApiRoles(row) {
      if (!isEmpty(row) && isEmpty(row.aliuid)) {
        this.apiRoleList = [USER_TYPE_RESEARCHER]
      } else {
        this.apiRoleList = [USER_TYPE_ADMIN, USER_TYPE_RESEARCHER]
      }
    },
    handleUpdate(row) {
      this.refreshUserList()
      this.refreshApiRoles(row)
      this.researcher = Object.assign({}, row)
      this.dialogStatus = 'update'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    handleDelete(row, index) {
      this.researcher = Object.assign({}, row)
      var tmpResearcher = this.resetUserGroupNameAndRoles(this.researcher, this.userGroupList)
      deleteResearcher(tmpResearcher).then(() => {
        this.list.splice(index, 1)
        this.$notify({
          title: this.$t('user.success'),
          message: this.$t('user.deleteSuccess'),
          type: 'success',
          duration: 2000
        })
      }).catch((error) => {
        this.$notify({
          title: this.$t('user.retry'),
          message: this.$t('user.deleteFailed') + ',' + error,
          type: 'error',
          duration: 2000
        })
      }).finally(() => {
        this.dialogFormVisible = false
        this.listLoading = false
      })
    },
    copyBearerToken(researcher) {
      getBearerTokenByUser(researcher).then((response) => {
        if (!response || !response.data || response.data.code !== 10000) {
          this.$notify({
            title: this.$t('user.getTokenFailed'),
            message: response.data,
            type: 'error',
            duration: 2000
          })
          return
        }
        const el = document.createElement('textarea')
        el.value = response.data.data
        document.body.appendChild(el)
        el.setAttribute('type', 'text')
        el.select()
        var successful = document.execCommand('copy')
        document.body.removeChild(el)
        if (successful) {
          this.$message({
            message: this.$t('user.tokenCopyedSuccess'),
            type: 'success'
          })
          return
        } else {
          this.$message({
            message: this.$t('user.tokenCopyedFailed'),
            type: 'error'
          })
        }
      }).catch((err) => {
        this.$notify({
          title: this.$t('user.refresh'),
          message: err,
          type: 'error',
          duration: 2000
        })
      })
    },
    downloadKubeConfig(researcher) {
      downloadK8sConfig(researcher).then((response) => {
        var suggestedFileName = response.headers['x-suggested-filename'].split(',')[0]
        var fileURL = window.URL.createObjectURL(new Blob([response.data]))
        var fileLink = document.createElement('a')
        fileLink.href = fileURL
        fileLink.setAttribute('download', suggestedFileName)
        document.body.appendChild(fileLink)
        fileLink.click()
      })
    },
    resetUserGroupNameAndRoles(researcher, groupList) {
      var res = JSON.parse(JSON.stringify(researcher))
      if (isEmpty(researcher.groups)) {
        return res
      }
      res.groups = groupList.filter(x => researcher.groups.indexOf(x.spec.groupName) >= 0).map(x => x.metadata.name)
      return res
    },
    createData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          var tmpResearcher = this.resetUserGroupNameAndRoles(this.researcher, this.userGroupList)
          createResearcher(tmpResearcher).then((response) => {
            if (response !== null && undefined !== response.data) {
              var createdResearcher = deserializeK8sUser(response.data)
              this.list.unshift(createdResearcher)
            }
            this.dialogFormVisible = false
            this.fetchData()
            this.$notify({
              title: this.$t('user.success'),
              message: this.$t('user.createSuccess'),
              type: 'success',
              duration: 2000
            })
          }).catch((error) => {
            this.$notify({
              title: this.$t('user.refresh'),
              message: this.$t('user.createFailed') + ',' + error,
              type: 'error',
              duration: 2000
            })
          }).finally(() => {
            this.dialogFormVisible = false
            this.listLoading = false
          })
        }
      })
    },
    updateData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          var tmpResearcher = this.resetUserGroupNameAndRoles(this.researcher, this.userGroupList)
          updateResearcher(tmpResearcher).then(() => {
            const index = this.list.findIndex(
              (v) => v.uid === this.researcher.uid
            )
            this.list.splice(index, 1, this.researcher)
            this.fetchData()
            this.$notify({
              title: this.$t('user.success'),
              message: this.$t('user.updateSuccess'),
              type: 'success',
              duration: 2000
            })
          }).catch((error) => {
            this.$notify({
              title: this.$t('user.refresh'),
              message: this.$t('user.updateFailed') + ',' + error,
              type: 'error',
              duration: 2000
            })
          }).finally(() => {
            this.dialogFormVisible = false
            this.listLoading = false
          })
        }
      })
    }
  }
}
</script>

<style lang="scss" scoped>
.NamespaceRole {
  //display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 5px;
  margin-bottom: 5px;
  //height: 100vh;
}
.el-select {
  display: block;
  //padding-right: 8px;
}
</style>
