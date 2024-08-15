/// <reference path="../pb_data/types.d.ts" />
migrate((db) => {
  const dao = new Dao(db)
  const collection = dao.findCollectionByNameOrId("_pb_users_auth_")

  collection.indexes = [
    "CREATE UNIQUE INDEX `idx_n8vxADZ` ON `users` (`tg_username`)",
    "CREATE UNIQUE INDEX `idx_8WqYACK` ON `users` (`tg_username`)"
  ]

  return dao.saveCollection(collection)
}, (db) => {
  const dao = new Dao(db)
  const collection = dao.findCollectionByNameOrId("_pb_users_auth_")

  collection.indexes = [
    "CREATE UNIQUE INDEX `idx_n8vxADZ` ON `users` (`tg_username`)"
  ]

  return dao.saveCollection(collection)
})
