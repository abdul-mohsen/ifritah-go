package handlers

func (h handler*) GetCarPart() {
	var id string 
	query = `
	select * from articlesvehicletrees a join
	articles on articles. a.linkingTargetId=? and  articles.legacyArticleId = a.legacyArticleId
	`


}
